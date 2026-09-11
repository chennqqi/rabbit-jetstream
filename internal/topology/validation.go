package topology

import "strings"

// ValidationIssue identifies a semantic error using a JSON Pointer into the
// submitted document, before canonical sorting. Messages remain human-readable.
type ValidationIssue struct {
	Path    string `json:"path"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ValidationError struct {
	Issues []ValidationIssue
}

func (e *ValidationError) Error() string {
	messages := make([]string, len(e.Issues))
	for i, issue := range e.Issues {
		messages[i] = issue.Message
	}
	return "invalid Queue: " + strings.Join(messages, "; ")
}
