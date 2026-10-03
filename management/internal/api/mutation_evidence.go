package api

import "net/http"

// This describes only this handler invocation's Queue/Stream/Consumer and
// declaration effects. It excludes audit/lock metadata and earlier replays.
// It is never an idempotency or durable outcome certificate.
type mutationEvidence struct {
	SchemaVersion   string `json:"schemaVersion"`
	Scope           string `json:"scope"`
	Phase           string `json:"phase"`
	ResourceEffects string `json:"resourceEffects"`
	IntentID        string `json:"intentId,omitempty"`
}

func writeMutationError(w http.ResponseWriter, status int, code, message, phase, intentID string) {
	effects := "possible"
	if phase == "audit_intent" {
		effects = "none"
	}
	writeJSON(w, status, map[string]any{"error": map[string]any{
		"code": code, "message": message,
		"mutation": mutationEvidence{SchemaVersion: "rjs.mutation-evidence.v1", Scope: "receiving-attempt", Phase: phase, ResourceEffects: effects, IntentID: intentID},
	}})
}
