package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const connectionFixture = `{"server_id":"node","now":"2026-09-10T00:00:00Z","num_connections":1,"total":201,"offset":0,"limit":50,"connections":[{"cid":18446744073709551615,"in_msgs":9007199254740993,"authorized_user":"secret-user","jwt":"secret-token","ip":"secret-ip","name":"secret-name","subscriptions_list":["secret-subject"]}]}`

func TestConnectionPageProjectionAndQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/connz" || r.URL.RawQuery != "auth=false&limit=50&offset=0&sort=cid&state=open&subs=false" {
			t.Errorf("unexpected request %s", r.URL)
		}
		fmt.Fprint(w, connectionFixture)
	}))
	defer server.Close()
	page, err := New(server.URL, time.Second).ConnectionPage(context.Background(), 0, "node", 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 201 || len(page.Items) != 1 || page.Items[0].CID != ^uint64(0) || *page.Items[0].InMessages != 9007199254740993 || page.Items[0].OutBytes != nil || page.ReadAt.IsZero() {
		t.Fatalf("invalid projection: %+v", page)
	}
	body, _ := json.Marshal(page)
	if strings.Contains(string(body), "secret") || strings.Contains(string(body), "authorized_user") {
		t.Fatalf("sensitive metadata projected: %s", body)
	}
}

func TestConnectionPageRejectsInvalidResponses(t *testing.T) {
	for _, body := range []string{
		`null`, `{}`, connectionFixture + ` {}`, strings.Replace(connectionFixture, `"node"`, `"other"`, 1),
		strings.Replace(connectionFixture, `"total":201`, `"total":0`, 1),
		strings.Replace(connectionFixture, `"offset":0`, `"offset":1`, 1),
		strings.Replace(connectionFixture, `"limit":50`, `"limit":200`, 1),
		strings.Replace(connectionFixture, `"num_connections":1`, `"num_connections":0`, 1),
		strings.Replace(connectionFixture, `9007199254740993`, `-1`, 1),
		strings.Replace(connectionFixture, `18446744073709551615`, `0`, 1),
		strings.Replace(strings.Replace(connectionFixture, `"num_connections":1`, `"num_connections":2`, 1), `"connections":[`, `"connections":[{"cid":18446744073709551615},`, 1),
		strings.Replace(strings.Replace(connectionFixture, `"num_connections":1`, `"num_connections":2`, 1), `"connections":[`, `"connections":[{"cid":18446744073709551615},`, 1) + ` trailing`,
		strings.Repeat(" ", maxConnectionResponseBytes) + connectionFixture,
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
		page, err := New(server.URL, time.Second).ConnectionPage(context.Background(), 0, "node", 0, 50)
		server.Close()
		if err == nil || page != nil {
			t.Fatal("invalid response accepted")
		}
		if strings.Contains(err.Error(), "secret") {
			t.Fatal("raw response leaked")
		}
	}
}

func TestConnectionPageRejectsRowsBeyondTotalAndTruncation(t *testing.T) {
	for _, truncated := range []bool{false, true} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body := strings.Replace(connectionFixture, `"offset":0`, `"offset":201`, 1)
			if truncated {
				w.Header().Set("Content-Length", fmt.Sprint(len(body)+1))
			}
			fmt.Fprint(w, body)
		}))
		page, err := New(server.URL, time.Second).ConnectionPage(context.Background(), 0, "node", 201, 50)
		server.Close()
		if err == nil || page != nil {
			t.Fatal("incomplete/inconsistent response accepted")
		}
	}
}

func TestConnectionPageEmptyOffsetAndRequestBounds(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		fmt.Fprint(w, `{"server_id":"node","now":"2026-09-10T00:00:00Z","num_connections":0,"total":1,"offset":200,"limit":50,"connections":[]}`)
	}))
	defer server.Close()
	c := New(server.URL, time.Second)
	page, err := c.ConnectionPage(context.Background(), 0, "node", 200, 50)
	if err != nil || page.Offset != 200 || page.Total != 1 {
		t.Fatalf("offset clamped: %v %+v", err, page)
	}
	for _, values := range [][3]int{{-1, 0, 50}, {1, 0, 50}, {0, -1, 50}, {0, 1000001, 50}, {0, 0, 0}, {0, 0, 201}} {
		if _, err := c.ConnectionPage(context.Background(), values[0], "node", values[1], values[2]); err == nil {
			t.Fatal("invalid request accepted")
		}
	}
	if calls.Load() != 1 {
		t.Fatal("invalid requests reached endpoint")
	}
}

func TestConnectionPageDoesNotFollowRedirectOrIgnoreCancellation(t *testing.T) {
	var leaked atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { leaked.Add(1) }))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, http.StatusFound) }))
	defer server.Close()
	c := New(server.URL, time.Second)
	if page, err := c.ConnectionPage(context.Background(), 0, "node", 0, 50); err == nil || page != nil {
		t.Fatal("redirect accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if page, err := c.ConnectionPage(ctx, 0, "node", 0, 50); err == nil || page != nil {
		t.Fatal("cancellation ignored")
	}
	if leaked.Load() != 0 {
		t.Fatal("redirect destination reached")
	}
}
