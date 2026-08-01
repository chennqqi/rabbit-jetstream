package topology

import "time"

const DeclarationAPIVersion = "rabbit-jetstream.io/declaration/v1alpha1"

type Declaration struct {
	APIVersion string    `json:"apiVersion"`
	Queue      string    `json:"queue"`
	Revision   string    `json:"revision"`
	Plan       Plan      `json:"plan"`
	AppliedAt  time.Time `json:"appliedAt"`
	KVRevision uint64    `json:"kvRevision,omitempty"`
}
