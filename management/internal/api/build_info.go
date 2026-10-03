package api

import (
	"context"
	"net/http"
	"regexp"
	"runtime"
	"runtime/debug"
	"time"

	adminui "github.com/chennqqi/rabbit-jetstream/admin-ui"
)

type consoleBuildInfo struct {
	SchemaVersion  string                `json:"schemaVersion"`
	Version        string                `json:"version"`
	GoVersion      string                `json:"goVersion"`
	OS             string                `json:"os"`
	Arch           string                `json:"arch"`
	Revision       string                `json:"revision,omitempty"`
	Modified       *bool                 `json:"modified,omitempty"`
	RevisionSource string                `json:"revisionSource,omitempty"`
	UIAssets       adminui.AssetIdentity `json:"uiAssets"`
}

var exactRevisionPattern = regexp.MustCompile(`^[a-f0-9]{40}$`)

func buildInfo(version, injectedRevision string, injectedClean bool, info *debug.BuildInfo) consoleBuildInfo {
	result := consoleBuildInfo{SchemaVersion: "rjs.build-info.v1", Version: version, GoVersion: runtime.Version(), OS: runtime.GOOS, Arch: runtime.GOARCH, UIAssets: adminui.EmbeddedAssetIdentity()}
	if exactRevisionPattern.MatchString(injectedRevision) {
		result.Revision, result.RevisionSource = injectedRevision, "injected"
		if injectedClean {
			clean := false
			result.Modified, result.RevisionSource = &clean, "release-build"
		}
		return result
	}
	if info != nil {
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				result.Revision = setting.Value
				result.RevisionSource = "go-build-info"
			case "vcs.modified":
				if setting.Value == "true" || setting.Value == "false" {
					value := setting.Value == "true"
					result.Modified = &value
				}
			}
		}
	}
	return result
}

// Build metadata is reported, not an artifact attestation. Never expose the
// complete build settings: linker flags and local paths can contain secrets.
func (h *Handler) consoleBuild(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	if !h.authorize(w, r, map[string]bool{"operator": true, "auditor": true}, "build_info_disabled", "build information") {
		return
	}
	if r.URL.RawQuery != "" {
		writeAPIError(w, 400, "invalid_query", "build information accepts no query parameters")
		return
	}
	info, _ := debug.ReadBuildInfo()
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, buildInfo(h.version, h.console.RuntimeRevision, h.console.RuntimeClean, info))
}
