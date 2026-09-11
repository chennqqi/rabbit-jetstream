package topology

import (
	"crypto/sha256"
	_ "embed"
	"fmt"
)

const QueueSchemaID = "urn:rabbit-jetstream:queue:v1alpha1:schema:1"
const QueueSchemaVersion = "rjs.queue-schema.v1"

//go:embed queue-schema.json
var queueSchema []byte

// QueueSchema returns an owned copy; callers cannot change the published schema.
func QueueSchema() []byte { return append([]byte(nil), queueSchema...) }

func QueueSchemaETag() string {
	return fmt.Sprintf("\"rjs-queue-schema-v1:%x\"", sha256.Sum256(queueSchema))
}
