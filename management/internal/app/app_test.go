package app

import (
	"net/http"
	"testing"
	"time"
)

func TestHTTPServerHasProductionResourceLimits(t *testing.T) {
	handler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	server := newHTTPServer(":8223", handler, 5*time.Second)
	if server.Addr != ":8223" || server.Handler == nil {
		t.Fatal("server address or handler was not retained")
	}
	if server.ReadHeaderTimeout != 5*time.Second || server.ReadTimeout != 15*time.Second || server.WriteTimeout != 30*time.Second || server.IdleTimeout != 60*time.Second {
		t.Fatalf("unexpected HTTP timeouts: header=%s read=%s write=%s idle=%s", server.ReadHeaderTimeout, server.ReadTimeout, server.WriteTimeout, server.IdleTimeout)
	}
	if server.MaxHeaderBytes != 64<<10 {
		t.Fatalf("MaxHeaderBytes=%d, want %d", server.MaxHeaderBytes, 64<<10)
	}
}
