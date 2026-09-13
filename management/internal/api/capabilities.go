package api

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/chennqqi/rabbit-jetstream/internal/topology"
)

// ConsoleConfig describes operator-declared intent, never observed capacity.
type ConsoleConfig struct {
	DeploymentProfile string
	RuntimeRevision   string
	RuntimeClean      bool
	Qualification     *QualificationReport
	History           HistoryBackend
	Alerts            AlertBackend
}

type QualificationReport struct {
	Statement      string
	ManifestDigest string
}

type consoleCapabilities struct {
	SchemaVersion string `json:"schemaVersion"`
	Deployment    struct {
		Profile string `json:"profile"`
		Source  string `json:"source"`
	} `json:"deployment"`
	Queue struct {
		Schema struct {
			ID      string `json:"id"`
			Version string `json:"version"`
			URL     string `json:"url"`
			ETag    string `json:"etag"`
		} `json:"schema"`
		APIVersion               string   `json:"apiVersion"`
		Kind                     string   `json:"kind"`
		SupportedReplicas        []int    `json:"supportedReplicas"`
		SupportedStorage         []string `json:"supportedStorage"`
		MinimumPriority          int      `json:"minimumPriority"`
		MaximumPriority          int      `json:"maximumPriority"`
		RequiresExplicitReplicas bool     `json:"requiresExplicitReplicas"`
		Defaults                 struct {
			Storage  string                  `json:"storage"`
			Delivery topology.DeliveryPolicy `json:"delivery"`
		} `json:"defaults"`
	} `json:"queue"`
	Features      []string `json:"features"`
	Qualification struct {
		Status         string `json:"status"`
		Statement      string `json:"statement,omitempty"`
		ManifestDigest string `json:"manifestDigest,omitempty"`
	} `json:"qualification"`
}

func (h *Handler) consoleCapabilities(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	if !h.authorize(w, r, map[string]bool{"operator": true, "auditor": true}, "capabilities_api_disabled", "console capabilities") {
		return
	}
	if r.URL.RawQuery != "" {
		writeAPIError(w, http.StatusBadRequest, "invalid_query", "capabilities accepts no query parameters")
		return
	}
	w.Header().Set("ETag", h.capabilitiesETag())
	writeJSON(w, http.StatusOK, h.capabilitiesContract())
}

func (h *Handler) capabilitiesContract() consoleCapabilities {
	result := consoleCapabilities{SchemaVersion: "rjs.console-capabilities.v1"}
	result.Deployment.Profile, result.Deployment.Source = "unknown", "unspecified"
	if h.console.DeploymentProfile == "standalone" || h.console.DeploymentProfile == "cluster" {
		result.Deployment.Profile, result.Deployment.Source = h.console.DeploymentProfile, "configuration"
	}
	result.Queue.APIVersion, result.Queue.Kind = topology.QueueAPIVersion, topology.QueueKind
	result.Queue.Schema.ID, result.Queue.Schema.Version = topology.QueueSchemaID, topology.QueueSchemaVersion
	result.Queue.Schema.URL, result.Queue.Schema.ETag = "/api/v1/console/queue-schema", topology.QueueSchemaETag()
	result.Queue.SupportedReplicas, result.Queue.SupportedStorage = []int{1, 3, 5}, []string{"file", "memory"}
	result.Queue.MinimumPriority, result.Queue.MaximumPriority = topology.MinimumPriority, topology.MaximumPriority
	result.Queue.RequiresExplicitReplicas = true
	var defaults topology.Queue
	defaults.Default()
	result.Queue.Defaults.Storage, result.Queue.Defaults.Delivery = defaults.Spec.Storage, defaults.Spec.Delivery
	// These identify implemented contracts, not permission, runtime readiness,
	// deployment capacity, complete JSON schema, or release qualification.
	result.Features = []string{"queue-document", "queue-preview", "queue-delete-preview", "conditional-queue-writes", "filtered-resource-lists", "request-audit-windows", "capability-preconditions", "queue-schema", "diagnostics-metadata-jobs", "prometheus-metric-history", "prometheus-operational-alerts", "authenticated-sse-invalidations"}
	result.Qualification.Status = "unreported"
	if h.console.Qualification != nil {
		result.Qualification.Status = "reported"
		result.Qualification.Statement = h.console.Qualification.Statement
		result.Qualification.ManifestDigest = h.console.Qualification.ManifestDigest
	}
	return result
}

const capabilitiesMatchHeader = "X-RJS-If-Capabilities-Match"

var capabilitiesTagPattern = regexp.MustCompile(`^"rjs-capabilities-v1:[0-9a-f]{64}"$`)

func (h *Handler) capabilitiesETag() string {
	// Only immutable process configuration, build version and contract data.
	// Name, uptime, credentials and live resource observations are excluded.
	data, err := json.Marshal(struct {
		Version  string
		Contract consoleCapabilities
	}{h.version, h.capabilitiesContract()})
	if err != nil {
		panic(err)
	} // Struct contains only fixed JSON-serializable fields.
	return fmt.Sprintf("\"rjs-capabilities-v1:%x\"", sha256.Sum256(data))
}

func (h *Handler) checkCapabilitiesPrecondition(w http.ResponseWriter, r *http.Request) bool {
	values := r.Header.Values(capabilitiesMatchHeader)
	if len(values) == 0 {
		return true
	} // Additive compatibility for existing clients.
	if len(values) != 1 || !capabilitiesTagPattern.MatchString(values[0]) {
		writeAPIError(w, http.StatusBadRequest, "invalid_capabilities_precondition", "one original capabilities ETag required")
		return false
	}
	if values[0] != h.capabilitiesETag() {
		writeAPIError(w, http.StatusPreconditionFailed, "capabilities_changed", "receiving server contract differs; refresh capabilities and preview")
		return false
	}
	return true
}
