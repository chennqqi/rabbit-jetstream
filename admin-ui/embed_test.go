package adminui

import (
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"
)

func TestEmbeddedAssetIdentity(t *testing.T) {
	identity := EmbeddedAssetIdentity()
	if identity.Algorithm != "sha256-framed-files-v1" || identity.FileCount < 4 {
		t.Fatalf("unexpected identity: %+v", identity)
	}
	decoded, err := hex.DecodeString(identity.Digest)
	if err != nil || len(decoded) != 32 {
		t.Fatalf("invalid SHA-256 digest %q: %v", identity.Digest, err)
	}
	if identity != EmbeddedAssetIdentity() {
		t.Fatal("embedded asset identity is not stable")
	}
}

func TestAssetIdentityBindsPathsLengthsAndContents(t *testing.T) {
	base := fstest.MapFS{"index.html": {Data: []byte("ab")}, "assets/app.js": {Data: []byte("c")}}
	sameDifferentInsertionOrder := fstest.MapFS{"assets/app.js": {Data: []byte("c")}, "index.html": {Data: []byte("ab")}}
	pathChanged := fstest.MapFS{"index.html": {Data: []byte("ab")}, "assets/other.js": {Data: []byte("c")}}
	boundaryChanged := fstest.MapFS{"index.html": {Data: []byte("a")}, "assets/app.js": {Data: []byte("bc")}}
	contentChanged := fstest.MapFS{"index.html": {Data: []byte("ax")}, "assets/app.js": {Data: []byte("c")}}

	want, err := identifyAssets(base)
	if err != nil {
		t.Fatal(err)
	}
	for name, files := range map[string]fstest.MapFS{"insertion order": sameDifferentInsertionOrder, "path": pathChanged, "boundary": boundaryChanged, "content": contentChanged} {
		got, err := identifyAssets(files)
		if err != nil {
			t.Fatal(err)
		}
		if name == "insertion order" && got != want {
			t.Fatalf("walk order changed identity: got %+v want %+v", got, want)
		}
		if name != "insertion order" && got.Digest == want.Digest {
			t.Errorf("%s change did not change digest", name)
		}
	}
}

func TestHandlerServesConsoleAndAssets(t *testing.T) {
	for _, test := range []struct{ path, content string }{
		{"/admin/", "Rabbit JetStream"},
		{"/admin/queues/example", "Rabbit JetStream"},
	} {
		recorder := httptest.NewRecorder()
		Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, test.path, nil))
		if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), test.content) {
			t.Fatalf("%s: status=%d body=%q", test.path, recorder.Code, recorder.Body.String())
		}
		if recorder.Header().Get("Content-Security-Policy") == "" {
			t.Fatalf("%s: missing CSP", test.path)
		}
		for header, expected := range map[string]string{
			"Cross-Origin-Resource-Policy": "same-origin",
			"Permissions-Policy":           "camera=(), microphone=(), geolocation=()",
			"Referrer-Policy":              "no-referrer",
			"X-Content-Type-Options":       "nosniff",
			"X-Frame-Options":              "DENY",
		} {
			if actual := recorder.Header().Get(header); actual != expected {
				t.Fatalf("%s: %s=%q, want %q", test.path, header, actual, expected)
			}
		}
	}
}

func TestHandlerServesEveryEntrypointAsset(t *testing.T) {
	index, err := assets.ReadFile("dist/index.html")
	if err != nil {
		t.Fatal(err)
	}
	matches := regexp.MustCompile(`(?:src|href)="(/admin/assets/[^"]+)"`).FindAllStringSubmatch(string(index), -1)
	if len(matches) < 3 {
		t.Fatalf("index has %d entrypoint assets, want script, vendor and stylesheet", len(matches))
	}
	for _, match := range matches {
		recorder := httptest.NewRecorder()
		Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, match[1], nil))
		if recorder.Code != http.StatusOK || recorder.Body.Len() == 0 {
			t.Errorf("%s: status=%d bytes=%d", match[1], recorder.Code, recorder.Body.Len())
		}
		if cache := recorder.Header().Get("Cache-Control"); cache != "public, max-age=3600" {
			t.Errorf("%s: Cache-Control=%q", match[1], cache)
		}
	}
}

func TestHandlerRejectsTraversal(t *testing.T) {
	recorder := httptest.NewRecorder()
	Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/admin/..%2fsecret", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status=%d", recorder.Code)
	}
}
