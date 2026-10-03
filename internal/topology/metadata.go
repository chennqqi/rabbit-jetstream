package topology

import "strings"

// DesiredMetadata replaces RJS-owned keys and preserves unrelated observations.
// This is shared by reconciliation and apply so their metadata scope agrees.
// It does not provide broker-side concurrency control for external writers.
func DesiredMetadata(observed, desired map[string]string) map[string]string {
	result := make(map[string]string, len(observed)+len(desired))
	for key, value := range observed {
		if !strings.HasPrefix(key, "rabbit-jetstream.io/") {
			result[key] = value
		}
	}
	for key, value := range desired {
		result[key] = value
	}
	return result
}
