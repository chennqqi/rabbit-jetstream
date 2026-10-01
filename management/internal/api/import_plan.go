package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"

	"github.com/chennqqi/rabbit-jetstream/internal/topology"
	"github.com/chennqqi/rabbit-jetstream/management/internal/jetstream"
)

type importDependencyObservation struct {
	Queue  string `json:"queue"`
	Status string `json:"status"`
	ETag   string `json:"etag,omitempty"`
}

func (h *Handler) planQueueImport(w http.ResponseWriter, r *http.Request) {
	if !h.authorizeWrite(w, r) || !h.checkCapabilitiesPrecondition(w, r) {
		return
	}
	if r.URL.RawQuery != "" {
		writeAPIError(w, 400, "invalid_query", "import planning accepts no query parameters")
		return
	}
	defer r.Body.Close()
	var input struct {
		Documents []json.RawMessage `json:"documents"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20))
	start, startErr := decoder.Token()
	key, keyErr := decoder.Token()
	if startErr != nil || keyErr != nil || start != json.Delim('{') || key != "documents" {
		writeAPIError(w, 400, "invalid_import", "invalid or oversized import JSON envelope")
		return
	}
	if err := decoder.Decode(&input.Documents); err != nil {
		writeAPIError(w, 400, "invalid_import", "invalid documents array")
		return
	}
	end, endErr := decoder.Token()
	if endErr != nil || end != json.Delim('}') {
		writeAPIError(w, 400, "invalid_import", "unknown or repeated envelope field")
		return
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) || len(input.Documents) == 0 || len(input.Documents) > topology.MaximumImportQueues {
		writeAPIError(w, 400, "invalid_import", "supply exactly one envelope with 1–100 documents")
		return
	}
	queues := make([]topology.Queue, len(input.Documents))
	for index, raw := range input.Documents {
		// Preserve safe identity even when the typed document fails validation,
		// so dependents are blocked instead of mistaking this item as external.
		var identity struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
		}
		_ = json.Unmarshal(raw, &identity)
		name := identity.Metadata.Name
		if !topology.ValidQueueName(name) || len(name) > 256 {
			continue
		}
		queues[index].Metadata.Name = name
		if len(raw) > 1<<20 {
			continue
		}
		queue, err := topology.ParseQueue(bytes.NewReader(raw))
		if err == nil && (queue.Spec.DeadLetter == nil || len(queue.Spec.DeadLetter.Queue) <= 256) {
			queues[index] = *queue
		}
	}
	plan, err := topology.PlanImport(queues)
	if err != nil {
		writeAPIError(w, 400, "invalid_import", "invalid import collection")
		return
	}
	external := map[string]bool{}
	for _, item := range plan.Items {
		for _, problem := range item.Problems {
			if problem.Code == "external_dependency_unverified" {
				external[problem.Dependency] = true
			}
		}
	}
	names := make([]string, 0, len(external))
	for name := range external {
		names = append(names, name)
	}
	sort.Strings(names)
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	observations := make([]importDependencyObservation, 0, len(names))
	for _, name := range names {
		if ctx.Err() != nil {
			writeAPIError(w, 503, "import_plan_unavailable", "dependency lookup canceled or deadline exceeded")
			return
		}
		item := importDependencyObservation{Queue: name, Status: "unavailable"}
		declaration, err := h.client.Declaration(ctx, name)
		switch {
		case errors.Is(err, jetstream.ErrNotFound):
			item.Status = "missing"
		case err != nil:
		case declaration == nil || declaration.Queue != name || declaration.Plan.Queue != name || declaration.Revision != declaration.Plan.Revision:
			item.Status = "unrepresentable"
		default:
			if _, err := topology.QueueDocument(declaration.Plan); err != nil {
				item.Status = "unrepresentable"
			} else {
				item.Status = "present"
				item.ETag = fmt.Sprintf("\"%d\"", declaration.KVRevision)
			}
		}
		observations = append(observations, item)
	}
	if ctx.Err() != nil {
		writeAPIError(w, 503, "import_plan_unavailable", "dependency lookup canceled or deadline exceeded")
		return
	}
	writeJSON(w, 200, struct {
		Plan     topology.ImportPlan           `json:"plan"`
		External []importDependencyObservation `json:"external_declarations"`
		Scope    string                        `json:"scope"`
	}{plan, observations, "declaration-only-not-apply-authorization"})
}
