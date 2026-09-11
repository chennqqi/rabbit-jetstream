package api

import (
	"errors"
	"net/http"

	"github.com/chennqqi/rabbit-jetstream/internal/topology"
)

func writeQueueValidationError(w http.ResponseWriter, err error) {
	detail := map[string]any{"code": "invalid_queue", "message": err.Error()}
	var validation *topology.ValidationError
	if errors.As(err, &validation) {
		detail["issues_version"] = "rjs.queue-validation.v1"
		detail["issues"] = validation.Issues
	}
	writeJSON(w, http.StatusBadRequest, map[string]any{"error": detail})
}
