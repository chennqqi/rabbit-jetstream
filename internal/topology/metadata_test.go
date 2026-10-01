package topology

import (
	"reflect"
	"testing"
)

func TestMetadataReconcilePresenceRemovalAndExternalPreservation(t *testing.T) {
	const prefix = "rabbit-jetstream.io/label."
	observed := map[string]string{prefix + "old": "", "external/key": "keep"}
	desired := map[string]string{prefix + "new": "", prefix + "literal": "(absent)"}
	effective := DesiredMetadata(observed, desired)
	if !reflect.DeepEqual(effective, map[string]string{prefix + "new": "", prefix + "literal": "(absent)", "external/key": "keep"}) {
		t.Fatalf("effective=%#v", effective)
	}
	op := Operation{}
	addMetadataChanges(&op, observed, desired)
	if len(op.Changes) != 3 {
		t.Fatalf("changes=%#v", op.Changes)
	}
	changes := map[string]Change{}
	for _, change := range op.Changes {
		changes[change.Path] = change
	}
	if change := changes["metadata."+prefix+"old"]; change.From != `""` || change.To != "(absent)" {
		t.Fatalf("empty removal=%#v", change)
	}
	if change := changes["metadata."+prefix+"new"]; change.From != "(absent)" || change.To != `""` {
		t.Fatalf("empty addition=%#v", change)
	}
	if change := changes["metadata."+prefix+"literal"]; change.To != `"(absent)"` {
		t.Fatalf("literal ambiguity=%#v", change)
	}
	effective["external/key"] = "changed"
	if observed["external/key"] != "keep" || desired[prefix+"new"] != "" {
		t.Fatal("input map mutated")
	}
	noop := Operation{}
	addMetadataChanges(&noop, map[string]string{"external/key": "keep"}, nil)
	if len(noop.Changes) != 0 {
		t.Fatalf("external-only drift changed: %#v", noop)
	}
}
