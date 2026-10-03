package monitoring

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNodeConnectionsIdentityResolution(t *testing.T) {
	for _, tc := range []struct {
		name, first, second, observed string
		want                          error
		reads                         int32
	}{
		{"unique", "node", "other", "node", nil, 1},
		{"missing", "other", "another", "node", ErrConnectionNodeMissing, 0},
		{"duplicate", "node", "node", "node", ErrConnectionNodeAmbiguous, 0},
		{"unknown uniqueness", "node", "", "node", ErrConnectionUnavailable, 0},
		{"unknown absence", "other", "", "node", ErrConnectionUnavailable, 0},
		{"restart", "node", "other", "restarted", ErrConnectionUnavailable, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var reads atomic.Int32
			makeServer := func(id string) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/varz" {
						fmt.Fprintf(w, `{"server_id":%q}`, id)
						return
					}
					if r.URL.Path != "/connz" || id != "node" {
						t.Errorf("unexpected target: %s %s", id, r.URL.Path)
					}
					reads.Add(1)
					body := strings.Replace(connectionFixture, `"server_id":"node"`, `"server_id":"`+tc.observed+`"`, 1)
					if r.URL.Query().Get("cid") != "" {
						body = strings.ReplaceAll(strings.ReplaceAll(body, `"limit":50`, `"limit":1`), `"total":201`, `"total":1`)
					}
					fmt.Fprint(w, body)
				}))
			}
			a, b := makeServer(tc.first), makeServer(tc.second)
			defer a.Close()
			defer b.Close()
			page, err := New(a.URL+","+b.URL, time.Second).NodeConnections(context.Background(), "node", 0, 50)
			if !errors.Is(err, tc.want) || (page == nil) != (tc.want != nil) || reads.Load() != tc.reads {
				t.Fatalf("page=%+v err=%v reads=%d", page, err, reads.Load())
			}
			detail, err := New(a.URL+","+b.URL, time.Second).NodeConnection(context.Background(), "node", ^uint64(0))
			if !errors.Is(err, tc.want) || (detail == nil) != (tc.want != nil) || reads.Load() != tc.reads*2 {
				t.Fatalf("detail=%+v err=%v reads=%d", detail, err, reads.Load())
			}
		})
	}
}

func TestNodeConnectionsLimitsConcurrentIdentityReads(t *testing.T) {
	var active, peak atomic.Int32
	release := make(chan struct{})
	var once sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := active.Add(1)
		defer active.Add(-1)
		for {
			previous := peak.Load()
			if current <= previous || peak.CompareAndSwap(previous, current) {
				break
			}
		}
		if current == 4 {
			once.Do(func() { close(release) })
		}
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		fmt.Fprint(w, `{"server_id":"other"}`)
	}))
	defer server.Close()
	_, err := New(strings.Repeat(server.URL+",", 12), time.Second).NodeConnections(context.Background(), "node", 0, 50)
	if !errors.Is(err, ErrConnectionNodeMissing) || peak.Load() != 4 {
		t.Fatalf("err=%v peak=%d active=%d", err, peak.Load(), active.Load())
	}
}

func TestNodeConnectionsHonorsCallerDeadline(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(started); <-r.Context().Done() }))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := New(server.URL, time.Second).NodeConnections(ctx, "node", 0, 50)
	if !errors.Is(err, ErrConnectionUnavailable) || ctx.Err() == nil {
		t.Fatalf("deadline not honored: %v", err)
	}
	select {
	case <-started:
	default:
		t.Fatal("deadline test did not exercise an in-flight request")
	}
}

func TestNodeConnectionsRejectsUntrustedIdentitySources(t *testing.T) {
	for _, body := range []string{`null`, `{}`, `{"server_id":"node"} trailing`, `{"server_id":" node"}`, strings.Repeat(" ", maxConnectionResponseBytes) + `{"server_id":"node"}`} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/varz" {
				t.Error("connection request after invalid identity")
			}
			fmt.Fprint(w, body)
		}))
		page, err := New(server.URL, time.Second).NodeConnections(context.Background(), "node", 0, 50)
		server.Close()
		if page != nil || !errors.Is(err, ErrConnectionUnavailable) {
			t.Fatalf("invalid identity accepted: %v", err)
		}
	}
}

func TestNodeConnectionsBoundsAndCancellation(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); fmt.Fprint(w, `{"server_id":"node"}`) }))
	defer server.Close()
	c := New(server.URL, time.Second)
	for _, id := range []string{"", " node", "node ", "https://host", "node\n", strings.Repeat("x", 257)} {
		if _, err := c.NodeConnections(context.Background(), id, 0, 50); !errors.Is(err, ErrConnectionQuery) {
			t.Fatal(err)
		}
	}
	for _, pair := range [][2]int{{-1, 50}, {1000001, 50}, {0, 0}, {0, 201}} {
		if _, err := c.NodeConnections(context.Background(), "node", pair[0], pair[1]); !errors.Is(err, ErrConnectionQuery) {
			t.Fatal(err)
		}
	}
	if _, err := New(strings.Repeat(server.URL+",", 33), time.Second).NodeConnections(context.Background(), "node", 0, 50); !errors.Is(err, ErrConnectionEndpointLimit) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.NodeConnections(ctx, "node", 0, 50); !errors.Is(err, ErrConnectionUnavailable) {
		t.Fatal(err)
	}
	if calls.Load() != 0 {
		t.Fatal("invalid/cancelled lookup issued HTTP request")
	}
	if _, err := New("", time.Second).NodeConnections(context.Background(), "node", 0, 50); !errors.Is(err, ErrConnectionUnavailable) {
		t.Fatal(err)
	}
}
