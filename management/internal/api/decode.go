package api

import (
	"encoding/json"
	"io"
	"net/http"
)

// decodeStrict reads exactly one JSON request value: the body is size-capped,
// unknown fields are rejected, and trailing content fails the decode. On
// failure it answers 400 with the caller's error code and returns false, so
// handlers add only their semantic validation afterwards. Handlers with a
// pre-decode content-type contract or multi-stage decoding keep bespoke paths.
func decodeStrict[T any](w http.ResponseWriter, r *http.Request, maxBytes int64, code, message string) (T, bool) {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBytes))
	decoder.DisallowUnknownFields()
	var request T
	if err := decoder.Decode(&request); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		writeAPIError(w, http.StatusBadRequest, code, message)
		return request, false
	}
	return request, true
}
