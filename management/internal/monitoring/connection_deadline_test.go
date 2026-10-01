package monitoring

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type connectionDeadlineTransport func(*http.Request) (*http.Response, error)

func (f connectionDeadlineTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestNodeConnectionSharesResolutionAndDetailDeadline(t *testing.T) {
	for _, callerLimit := range []time.Duration{time.Second, 10 * time.Second} {
		t.Run(callerLimit.String(), func(t *testing.T) {
			var deadlines []time.Time
			client := New("http://configured.invalid", time.Minute)
			client.http.Transport = connectionDeadlineTransport(func(r *http.Request) (*http.Response, error) {
				deadline, ok := r.Context().Deadline()
				if !ok {
					t.Fatal("request has no deadline")
				}
				deadlines = append(deadlines, deadline)
				body := `{"server_id":"node"}`
				if r.URL.Path == "/connz" {
					body = `{"server_id":"node","now":"2026-09-10T00:00:00Z","offset":0,"limit":1,"total":200,"num_connections":1,"connections":[{"cid":7}]}`
				}
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
			})
			ctx, cancel := context.WithTimeout(context.Background(), callerLimit)
			defer cancel()
			started := time.Now()
			detail, err := client.NodeConnection(ctx, "node", 7)
			if err != nil || detail == nil {
				t.Fatalf("read: %v", err)
			}
			if len(deadlines) != 2 || !deadlines[0].Equal(deadlines[1]) {
				t.Fatalf("resolution/detail deadlines differ: %v", deadlines)
			}
			if callerLimit < 5*time.Second {
				callerDeadline, _ := ctx.Deadline()
				if !deadlines[0].Equal(callerDeadline) {
					t.Fatal("earlier caller deadline lost")
				}
			} else if remaining := deadlines[0].Sub(started); remaining < 4*time.Second || remaining > 5*time.Second+100*time.Millisecond {
				t.Fatalf("wrong overall deadline: %v", remaining)
			}
		})
	}
}

func TestNodeConnectionCancellationDuringHeadersAndPartialBody(t *testing.T) {
	for _, partial := range []bool{false, true} {
		t.Run(fmt.Sprint(partial), func(t *testing.T) {
			started, aborted := make(chan struct{}), make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/varz" {
					fmt.Fprint(w, `{"server_id":"node"}`)
					return
				}
				if partial {
					w.Header().Set("Content-Length", "4096")
					fmt.Fprint(w, `{"server_id":`)
					w.(http.Flusher).Flush()
				}
				close(started)
				<-r.Context().Done()
				close(aborted)
			}))
			defer server.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			result := make(chan error, 1)
			go func() {
				detail, err := New(server.URL, 5*time.Second).NodeConnection(ctx, "node", 7)
				if detail != nil {
					result <- errors.New("partial observation returned")
					return
				}
				result <- err
			}()
			select {
			case <-started:
			case <-time.After(time.Second):
				t.Fatal("detail stage was not reached")
			}
			cancel()
			select {
			case err := <-result:
				if !errors.Is(err, ErrConnectionUnavailable) {
					t.Fatalf("cancel mapped to %v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("cancellation did not settle read")
			}
			select {
			case <-aborted:
			case <-time.After(time.Second):
				t.Fatal("upstream request was not canceled")
			}
		})
	}
}
