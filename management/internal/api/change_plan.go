package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/chennqqi/rabbit-jetstream/internal/topology"
	"github.com/chennqqi/rabbit-jetstream/management/internal/jetstream"
)

const maximumQueueChanges = 100

type queueChangePlanItem struct {
	Index   int                    `json:"index"`
	Queue   string                 `json:"queue"`
	ETag    string                 `json:"etag,omitempty"`
	Status  string                 `json:"status"`
	Code    string                 `json:"code,omitempty"`
	Preview *jetstream.PlanPreview `json:"preview,omitempty"`
}

// planQueueChanges is a bounded, read-only collection of the same per-Queue
// previews used by the editor. It deliberately has no apply or rollback path.
func (h *Handler) planQueueChanges(w http.ResponseWriter, r *http.Request) {
	if !h.authorizeWrite(w, r) || !h.checkCapabilitiesPrecondition(w, r) {
		return
	}
	if r.URL.RawQuery != "" {
		writeAPIError(w, 400, "invalid_query", "change planning accepts no query parameters")
		return
	}
	defer r.Body.Close()
	var envelope struct {
		Items []struct {
			Document json.RawMessage `json:"document"`
			ETag     string          `json:"etag"`
		} `json:"items"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&envelope); err != nil {
		writeAPIError(w, 400, "invalid_change_plan", "invalid or oversized change-plan envelope")
		return
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) || len(envelope.Items) < 1 || len(envelope.Items) > maximumQueueChanges {
		writeAPIError(w, 400, "invalid_change_plan", "supply exactly 1-100 change items")
		return
	}

	results := make([]queueChangePlanItem, len(envelope.Items))
	plans := make([]*topology.Plan, len(envelope.Items))
	revisions := make([]uint64, len(envelope.Items))
	counts := map[string]int{}
	for index, input := range envelope.Items {
		item := queueChangePlanItem{Index: index, Status: "invalid", Code: "invalid_declaration"}
		var identity struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
		}
		_ = json.Unmarshal(input.Document, &identity)
		if topology.ValidQueueName(identity.Metadata.Name) && len(identity.Metadata.Name) <= 256 {
			item.Queue = identity.Metadata.Name
			counts[item.Queue]++
		}
		results[index] = item
		if item.Queue == "" || len(input.Document) > 1<<20 {
			continue
		}
		queue, err := topology.ParseQueue(bytes.NewReader(input.Document))
		if err != nil || queue.Metadata.Name != item.Queue {
			continue
		}
		plan, err := topology.BuildPlan(*queue)
		if err != nil {
			continue
		}
		revision, ok := quotedRevision(input.ETag)
		if !ok {
			results[index].Code = "invalid_etag"
			continue
		}
		results[index].ETag = input.ETag
		results[index].Code = ""
		plans[index], revisions[index] = &plan, revision
	}
	for index := range results {
		if results[index].Queue != "" && counts[results[index].Queue] > 1 {
			results[index].Status, results[index].Code, plans[index] = "invalid", "duplicate_queue", nil
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	ready := true
	for index, plan := range plans {
		if plan == nil {
			ready = false
			continue
		}
		if ctx.Err() != nil {
			results[index].Status, results[index].Code, ready = "unavailable", "deadline_exceeded", false
			continue
		}
		preview, err := h.client.Preview(ctx, *plan, jetstream.ApplyPrecondition{ExpectedRevision: &revisions[index]})
		if err != nil {
			_, code := backendErrorDetails(err)
			results[index].Status, results[index].Code, ready = changePreviewStatus(code), code, false
			continue
		}
		results[index].Preview = preview
		if preview == nil || preview.CreateOnly || preview.BaseRevision != results[index].ETag || preview.Plan.Queue != results[index].Queue {
			results[index].Status, results[index].Code, results[index].Preview, ready = "invalid", "invalid_preview", nil, false
		} else if preview.Result.Blocked {
			results[index].Status, ready = "blocked", false
		} else {
			results[index].Status = "ready"
		}
	}
	writeJSON(w, 200, struct {
		Scope string                `json:"scope"`
		Ready bool                  `json:"ready"`
		Items []queueChangePlanItem `json:"items"`
	}{"bulk-change-preview-not-apply-authorization", ready, results})
}

func quotedRevision(value string) (uint64, bool) {
	if len(value) < 3 || value[0] != '"' || value[len(value)-1] != '"' || strings.Contains(value[1:len(value)-1], "\"") {
		return 0, false
	}
	digits := value[1 : len(value)-1]
	revision, err := strconv.ParseUint(digits, 10, 64)
	return revision, err == nil && revision > 0 && strconv.FormatUint(revision, 10) == digits
}

func changePreviewStatus(code string) string {
	switch code {
	case "dlq_dependency_cycle":
		return "blocked"
	case "conflict":
		return "conflict"
	case "not_found":
		return "missing"
	default:
		return "unavailable"
	}
}
