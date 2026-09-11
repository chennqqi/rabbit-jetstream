package jetstream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/chennqqi/rabbit-jetstream/internal/topology"
	jsapi "github.com/nats-io/nats.go/jetstream"
)

func dependencyPlan(t *testing.T, name, dependency string) topology.Plan {
	t.Helper()
	document, err := topology.QueueDocument(testQueuePlan(t, name))
	if err != nil {
		t.Fatal(err)
	}
	if dependency != "" {
		document.Spec.DeadLetter = &topology.DeadLetterPolicy{Queue: dependency}
	}
	plan, err := topology.BuildPlan(*document)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func TestDeadLetterDependencyChainCyclesAndFailures(t *testing.T) {
	for _, scenario := range []string{"valid", "source cycle", "existing cycle", "missing", "unrepresentable", "canceled during read"} {
		t.Run(scenario, func(t *testing.T) {
			client, backend := testClient()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			a, b := dependencyPlan(t, "a", "b"), dependencyPlan(t, "b", "")
			switch scenario {
			case "source cycle":
				b = dependencyPlan(t, "b", "source")
			case "existing cycle":
				b = dependencyPlan(t, "b", "a")
			case "unrepresentable":
				b.Consumer.Stream = "OTHER"
			}
			if err := client.persistDeclaration(ctx, a); err != nil {
				t.Fatal(err)
			}
			if scenario != "missing" {
				if err := client.persistDeclaration(ctx, b); err != nil {
					t.Fatal(err)
				}
			}
			reads := 0
			client.js = &previewReadJS{backend: backend, get: func(ctx context.Context, key string) (jsapi.KeyValueEntry, error) {
				reads++
				entry, err := backend.kv.Get(ctx, key)
				if scenario == "canceled during read" {
					cancel()
				}
				return entry, err
			}}
			chain, err := client.deadLetterDependencyChain(ctx, dependencyPlan(t, "source", "a"))
			if scenario == "valid" {
				if err != nil || len(chain) != 2 {
					t.Fatalf("chain=%v err=%v", chain, err)
				}
			} else if err == nil || chain != nil {
				t.Fatal("unsafe chain accepted")
			}
			if reads > 2 || scenario == "canceled during read" && reads != 1 {
				t.Fatalf("unexpected reads %d", reads)
			}
			if strings.Contains(scenario, "cycle") && !strings.Contains(err.Error(), "cycle") {
				t.Fatal(err)
			}
			if errors.Is(err, ErrDeadLetterCycle) != strings.Contains(scenario, "cycle") {
				t.Fatalf("incorrect cycle classification: %v", err)
			}
		})
	}
}

func TestDeadLetterDependencyChainBound(t *testing.T) {
	for _, size := range []int{maximumDeadLetterDependencies, maximumDeadLetterDependencies + 1} {
		client, backend := testClient()
		ctx := context.Background()
		for index := 0; index < size; index++ {
			next := ""
			if index+1 < size {
				next = fmt.Sprintf("q%03d", index+1)
			}
			if err := client.persistDeclaration(ctx, dependencyPlan(t, fmt.Sprintf("q%03d", index), next)); err != nil {
				t.Fatal(err)
			}
		}
		reads := 0
		client.js = &previewReadJS{backend: backend, get: func(ctx context.Context, key string) (jsapi.KeyValueEntry, error) {
			reads++
			return backend.kv.Get(ctx, key)
		}}
		chain, err := client.deadLetterDependencyChain(ctx, dependencyPlan(t, "source", "q000"))
		if size == maximumDeadLetterDependencies {
			if err != nil || len(chain) != size {
				t.Fatalf("%d: %v", size, err)
			}
		} else if err == nil || chain != nil || !strings.Contains(err.Error(), "exceeds") {
			t.Fatalf("%d: %v", size, err)
		}
		if reads != maximumDeadLetterDependencies {
			t.Fatalf("read limit=%d", reads)
		}
	}
}

func TestDeadLetterDependencyRechecksTransitiveRevision(t *testing.T) {
	client, backend := testClient()
	ctx := context.Background()
	for _, plan := range []topology.Plan{dependencyPlan(t, "b", ""), dependencyPlan(t, "a", "b")} {
		if _, err := client.Apply(ctx, plan); err != nil {
			t.Fatal(err)
		}
	}
	reads := 0
	client.js = &previewReadJS{backend: backend, get: func(ctx context.Context, key string) (jsapi.KeyValueEntry, error) {
		entry, err := backend.kv.Get(ctx, key)
		if key == "queues.b" {
			reads++
			if reads == 2 && err == nil {
				entry.(*fakeKVEntry).revision++
			}
		}
		return entry, err
	}}
	err := client.checkDeadLetterDependency(ctx, dependencyPlan(t, "source", "a"))
	if err == nil || !strings.Contains(err.Error(), "changed during observation") || reads != 2 {
		t.Fatalf("reads=%d err=%v", reads, err)
	}
}

func TestDeadLetterDependencyRequiresCurrentOwnedStream(t *testing.T) {
	for _, scenario := range []string{"valid", "unrelated metadata", "missing", "foreign", "subjects", "limits", "identity", "declaration revision", "unrepresentable", "read failure", "changed declaration", "canceled"} {
		t.Run(scenario, func(t *testing.T) {
			client, backend := testClient()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			target := testQueuePlan(t, "target")
			if _, err := client.Apply(ctx, target); err != nil {
				t.Fatal(err)
			}
			source := testQueuePlan(t, "source")
			source.DeadLetter = &topology.DeadLetterPlan{Queue: target.Queue, Stream: target.Stream.Name}
			readOnly := &previewReadJS{backend: backend}
			switch scenario {
			case "unrelated metadata":
				backend.streams[target.Stream.Name].info.Config.Metadata["external-owner"] = "preserved"
			case "declaration revision", "unrepresentable":
				record, err := client.Declaration(ctx, target.Queue)
				if err != nil {
					t.Fatal(err)
				}
				if scenario == "declaration revision" {
					record.Revision = "different"
				} else {
					record.Plan.Consumer.Stream = "OTHER"
				}
				encoded, err := json.Marshal(record)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := backend.kv.Put(ctx, "queues.target", encoded); err != nil {
					t.Fatal(err)
				}
			case "missing":
				delete(backend.streams, target.Stream.Name)
			case "foreign":
				backend.streams[target.Stream.Name].info.Config.Metadata = map[string]string{}
			case "subjects":
				backend.streams[target.Stream.Name].info.Config.Subjects = []string{"foreign.events"}
			case "limits":
				backend.streams[target.Stream.Name].info.Config.MaxMsgs++
			case "identity":
				backend.streams[target.Stream.Name].info.Config.Name = "OTHER"
			case "read failure":
				readOnly.streamErr = context.DeadlineExceeded
			case "changed declaration":
				reads := 0
				readOnly.get = func(ctx context.Context, key string) (jsapi.KeyValueEntry, error) {
					entry, err := backend.kv.Get(ctx, key)
					if key == "queues.target" {
						reads++
						if reads == 2 && err == nil {
							entry.(*fakeKVEntry).revision++
						}
					}
					return entry, err
				}
			case "canceled":
				cancel()
			}
			client.js = readOnly // All writes panic; the check must remain read-only.
			err := client.checkDeadLetterDependency(ctx, source)
			if (err != nil) != (scenario != "valid" && scenario != "unrelated metadata") {
				t.Fatalf("error=%v", err)
			}
			if _, exists := backend.streams[source.Stream.Name]; exists {
				t.Fatal("dependency check created source")
			}
		})
	}
}

func TestApplyRechecksDeadLetterStreamAfterPreview(t *testing.T) {
	client, backend := testClient()
	ctx := context.Background()
	target := testQueuePlan(t, "target")
	if _, err := client.Apply(ctx, target); err != nil {
		t.Fatal(err)
	}
	source := testQueuePlan(t, "source")
	source.DeadLetter = &topology.DeadLetterPlan{Queue: target.Queue, Stream: target.Stream.Name}
	if _, err := client.Preview(ctx, source, ApplyPrecondition{CreateOnly: true}); err != nil {
		t.Fatal(err)
	}
	delete(backend.streams, target.Stream.Name)
	if _, err := client.ApplyConditional(ctx, source, ApplyPrecondition{CreateOnly: true}); err == nil {
		t.Fatal("accepted missing target after preview")
	}
	if _, exists := backend.streams[source.Stream.Name]; exists {
		t.Fatal("created source despite missing target")
	}
}
