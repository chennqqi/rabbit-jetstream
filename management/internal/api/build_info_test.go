package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"runtime/debug"
	"strings"
	"testing"
)

func TestBuildInfoAllowlist(t *testing.T) {
	info := buildInfo("candidate", "", false, &debug.BuildInfo{Path: "private/path", Settings: []debug.BuildSetting{{Key: "-ldflags", Value: "secret"}, {Key: "vcs.revision", Value: "abc"}, {Key: "vcs.modified", Value: "true"}}})
	encoded, _ := json.Marshal(info)
	if strings.Contains(string(encoded), "secret") || strings.Contains(string(encoded), "private") || info.Revision != "abc" || info.Modified == nil || !*info.Modified || info.UIAssets.Digest == "" || info.UIAssets.FileCount < 1 {
		t.Fatalf("%s", encoded)
	}
	if buildInfo("dev", "", false, nil).Modified != nil {
		t.Fatal("missing metadata is not clean-build proof")
	}
	revision := strings.Repeat("a", 40)
	injected := buildInfo("v1.0.0", revision, true, &debug.BuildInfo{Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: strings.Repeat("b", 40)}, {Key: "vcs.modified", Value: "true"}}})
	if injected.Revision != revision || injected.Modified == nil || *injected.Modified || injected.RevisionSource != "release-build" {
		t.Fatalf("injected identity not authoritative: %+v", injected)
	}
	untrusted := buildInfo("v1.0.0", revision, false, nil)
	if untrusted.Revision != revision || untrusted.Modified != nil || untrusted.RevisionSource != "injected" {
		t.Fatalf("ordinary injection claimed a clean release: %+v", untrusted)
	}
}

func TestBuildInfoAuthenticatedWithoutBroker(t *testing.T) {
	revision := strings.Repeat("c", 40)
	h := NewWithControllerAuth(&fakeBackend{err: errors.New("offline")}, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "candidate", nil, nil, AuthConfig{OperatorTokens: []string{"operator"}, AuditorTokens: []string{"auditor"}}, ConsoleConfig{RuntimeRevision: revision, RuntimeClean: true})
	for _, tc := range []struct {
		token, path string
		status      int
	}{{"", "/api/v1/console/build", 401}, {"operator", "/api/v1/console/build", 200}, {"auditor", "/api/v1/console/build", 200}, {"auditor", "/api/v1/console/build?x=1", 400}} {
		req := httptest.NewRequest("GET", tc.path, nil)
		if tc.token != "" {
			req.Header.Set("Authorization", "Bearer "+tc.token)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != tc.status {
			t.Fatalf("%s %d %s", tc.path, rec.Code, rec.Body.String())
		}
		if rec.Code == 200 {
			var info consoleBuildInfo
			if err := json.Unmarshal(rec.Body.Bytes(), &info); err != nil {
				t.Fatal(err)
			}
			if info.Version != "candidate" || info.SchemaVersion != "rjs.build-info.v1" || info.GoVersion == "" || info.Revision != revision || info.RevisionSource != "release-build" || info.Modified == nil || *info.Modified || info.UIAssets.Algorithm != "sha256-framed-files-v1" || rec.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("%+v", info)
			}
		}
	}
}
