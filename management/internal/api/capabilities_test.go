package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/chennqqi/rabbit-jetstream/internal/topology"
)

func TestConsoleCapabilities(t *testing.T) {
	for _, tc := range []struct {
		name, token, profile, query string
		auth                        AuthConfig
		status                      int
	}{
		{"operator", "secret", "cluster", "", AuthConfig{OperatorTokens: []string{"secret"}}, 200},
		{"auditor", "secret", "standalone", "", AuthConfig{AuditorTokens: []string{"secret"}}, 200},
		{"unknown", "secret", "", "", AuthConfig{OperatorTokens: []string{"secret"}}, 200},
		{"missing", "", "", "", AuthConfig{OperatorTokens: []string{"secret"}}, 401},
		{"disabled local demo", "", "", "", AuthConfig{}, 404},
		{"query", "secret", "", "?force=true", AuthConfig{OperatorTokens: []string{"secret"}}, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Nil backend/monitor proves capabilities does not depend on live NATS.
			h := NewWithControllerAuth(nil, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil, nil, tc.auth, ConsoleConfig{DeploymentProfile: tc.profile})
			r := httptest.NewRequest("GET", "/api/v1/console/capabilities"+tc.query, nil)
			if tc.token != "" {
				r.Header.Set("Authorization", "Bearer "+tc.token)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			if strings.Contains(w.Body.String(), "secret") || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("unsafe response")
			}
			if w.Code != 200 {
				return
			}
			var result consoleCapabilities
			if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			profile, source := tc.profile, "configuration"
			if profile == "" {
				profile, source = "unknown", "unspecified"
			}
			if result.Deployment.Profile != profile || result.Deployment.Source != source || result.Qualification.Status != "unreported" {
				t.Fatalf("intent=%#v", result)
			}
			var defaults topology.Queue
			defaults.Default()
			if result.Queue.Defaults.Storage != defaults.Spec.Storage || !reflect.DeepEqual(result.Queue.Defaults.Delivery, defaults.Spec.Delivery) || !result.Queue.RequiresExplicitReplicas {
				t.Fatal("defaults diverged")
			}
			for _, replicas := range result.Queue.SupportedReplicas {
				for _, storage := range result.Queue.SupportedStorage {
					q := defaults
					q.APIVersion = topology.QueueAPIVersion
					q.Kind = topology.QueueKind
					q.Metadata.Name = "check"
					q.Spec.Subjects = []string{"check.events"}
					q.Spec.Replicas = replicas
					q.Spec.Storage = storage
					for _, priority := range []int{result.Queue.MinimumPriority, result.Queue.MaximumPriority} {
						q.Spec.MaxPriority = &priority
						if err := q.Validate(); err != nil {
							t.Fatal(err)
						}
					}
				}
			}
		})
	}
}

func TestConsoleCapabilitiesReportsBoundManifestWithoutPath(t *testing.T) {
	const digest = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	h := NewWithControllerAuth(nil, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil, nil, AuthConfig{OperatorTokens: []string{"secret"}}, ConsoleConfig{Qualification: &QualificationReport{Statement: "native-linux-soak-and-canary-required", ManifestDigest: digest}})
	r := httptest.NewRequest("GET", "/api/v1/console/capabilities", nil)
	r.Header.Set("Authorization", "Bearer secret")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var result consoleCapabilities
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Qualification.Status != "reported" || result.Qualification.Statement != "native-linux-soak-and-canary-required" || result.Qualification.ManifestDigest != digest {
		t.Fatalf("qualification=%#v", result.Qualification)
	}
	if strings.Contains(w.Body.String(), "release-manifest.json") {
		t.Fatal("response exposed manifest path")
	}
}
