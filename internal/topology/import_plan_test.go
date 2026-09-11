package topology

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

func importQueue(name, dependency string) Queue {
	queue := Queue{APIVersion: QueueAPIVersion, Kind: QueueKind, Metadata: Metadata{Name: name}, Spec: QueueSpec{Replicas: 1, Subjects: []string{name + ".events"}}}
	if dependency != "" {
		queue.Spec.DeadLetter = &DeadLetterPolicy{Queue: dependency}
	}
	return queue
}

func importOrder(plan ImportPlan) []string {
	result := []string{}
	for _, index := range plan.Order {
		result = append(result, plan.Items[index].Queue)
	}
	return result
}

func TestPlanImportDeterministicDependencies(t *testing.T) {
	queues := []Queue{importQueue("source", "failed"), importQueue("z", ""), importQueue("failed", "archive"), importQueue("archive", "")}
	for _, input := range [][]Queue{queues, {queues[3], queues[2], queues[1], queues[0]}} {
		plan, err := PlanImport(input)
		if err != nil || !plan.Ready || !reflect.DeepEqual(importOrder(plan), []string{"archive", "failed", "source", "z"}) {
			t.Fatalf("plan=%+v error=%v", plan, err)
		}
		for index, item := range plan.Items {
			if item.Index != index || item.Queue != input[index].Metadata.Name || item.Revision == "" || len(item.Problems) != 0 {
				t.Fatalf("bad item: %+v", item)
			}
		}
		if !reflect.DeepEqual(plan.Order, plan.ReviewOrder) {
			t.Fatal("internally ready plans must have identical orders")
		}
	}
}

func TestPlanImportBlocksCyclesAndTheirDependents(t *testing.T) {
	queues := []Queue{importQueue("a", "b"), importQueue("b", "c"), importQueue("c", "a"), importQueue("source", "a"), importQueue("upstream", "source"), importQueue("independent", "")}
	plan, err := PlanImport(queues)
	if err != nil || plan.Ready || !reflect.DeepEqual(importOrder(plan), []string{"independent"}) {
		t.Fatalf("%+v %v", plan, err)
	}
	for index, item := range plan.Items[:5] {
		want := "dependency_cycle"
		if index >= 3 {
			want = "blocked_dependency"
		}
		if len(item.Problems) != 1 || item.Problems[0].Code != want {
			t.Fatalf("%s: %+v", item.Queue, item.Problems)
		}
	}
}

func TestPlanImportDuplicatesInvalidAndExternal(t *testing.T) {
	invalid := importQueue("invalid", "")
	invalid.Spec.Replicas = 2
	queues := []Queue{importQueue("duplicate", ""), importQueue("duplicate", ""), importQueue("dup_user", "duplicate"), invalid, importQueue("bad_user", "invalid"), importQueue("external_user", "outside"), importQueue("transitive", "external_user"), importQueue("good", "")}
	plan, err := PlanImport(queues)
	if err != nil || plan.Ready || !reflect.DeepEqual(importOrder(plan), []string{"good"}) {
		t.Fatalf("%+v %v", plan, err)
	}
	wants := []string{"duplicate_queue", "duplicate_queue", "blocked_dependency", "invalid_declaration", "blocked_dependency", "external_dependency_unverified", "blocked_dependency"}
	for index, want := range wants {
		if len(plan.Items[index].Problems) != 1 || plan.Items[index].Problems[0].Code != want {
			t.Fatalf("%d: %+v", index, plan.Items[index])
		}
	}
	if plan.Items[5].Problems[0].Dependency != "outside" {
		t.Fatal("external identity lost")
	}
}

func TestPlanImportBoundsAndLongChain(t *testing.T) {
	for _, size := range []int{0, MaximumImportQueues + 1} {
		if _, err := PlanImport(make([]Queue, size)); err == nil {
			t.Fatalf("accepted %d", size)
		}
	}
	queues := make([]Queue, MaximumImportQueues)
	for index := range queues {
		dependency := ""
		if index+1 < len(queues) {
			dependency = fmt.Sprintf("q%03d", index+1)
		}
		queues[index] = importQueue(fmt.Sprintf("q%03d", index), dependency)
	}
	plan, err := PlanImport(queues)
	if err != nil || !plan.Ready || len(plan.Order) != len(queues) {
		t.Fatalf("%+v %v", plan, err)
	}
	for index, entry := range plan.Order {
		if entry != len(queues)-1-index {
			t.Fatal("dependency after dependent")
		}
	}
}

func TestPlanImportDoesNotMutateOrExposeDocuments(t *testing.T) {
	queue := importQueue("orders", "")
	queue.Metadata.Labels = map[string]string{"private": "not-in-result"}
	queue.Spec.Subjects = nil
	queue.Spec.Bindings = []Binding{{Exchange: "z", Type: "direct", Keys: []string{"z", "a"}}, {Exchange: "a", Type: "fanout"}}
	before, _ := json.Marshal(queue)
	plan, err := PlanImport([]Queue{queue})
	if err != nil || !plan.Ready {
		t.Fatalf("%+v %v", plan, err)
	}
	after, _ := json.Marshal(queue)
	if string(before) != string(after) {
		t.Fatal("caller slices reordered")
	}
	var projected map[string]any
	raw, _ := json.Marshal(plan.Items[0])
	if err := json.Unmarshal(raw, &projected); err != nil {
		t.Fatal(err)
	}
	if len(projected) != 5 {
		t.Fatalf("unexpected projection: %s", raw)
	}
	for _, key := range []string{"index", "queue", "revision", "dependencies", "problems"} {
		if _, ok := projected[key]; !ok {
			t.Fatalf("missing %s", key)
		}
	}
	if plan.Items[0].Dependencies == nil || plan.Items[0].Problems == nil {
		t.Fatal("nil collections")
	}
}

func TestPlanImportReviewOrderRetainsExternalProblemsAndStructuralBlocks(t *testing.T) {
	invalid := importQueue("invalid", "")
	invalid.Spec.Replicas = 2
	queues := []Queue{importQueue("source", "middle"), importQueue("middle", "outside"), importQueue("a", "b"), importQueue("b", "a"), importQueue("cycle_user", "a"), invalid, importQueue("bad_user", "invalid"), importQueue("duplicate", ""), importQueue("duplicate", ""), importQueue("dup_user", "duplicate"), importQueue("independent", "")}
	for pass := 0; pass < 2; pass++ {
		plan, err := PlanImport(queues)
		if err != nil {
			t.Fatal(err)
		}
		if plan.Ready || !reflect.DeepEqual(importOrder(plan), []string{"independent"}) {
			t.Fatalf("readiness changed: %+v", plan)
		}
		review := []string{}
		for _, index := range plan.ReviewOrder {
			review = append(review, plan.Items[index].Queue)
		}
		if !reflect.DeepEqual(review, []string{"independent", "middle", "source"}) {
			t.Fatalf("review order=%v", review)
		}
		for _, item := range plan.Items {
			if item.Queue == "middle" && (len(item.Problems) != 1 || item.Problems[0].Code != "external_dependency_unverified") {
				t.Fatal("external warning cleared")
			}
			if item.Queue == "source" && (len(item.Problems) != 1 || item.Problems[0].Code != "blocked_dependency") {
				t.Fatal("dependent warning cleared")
			}
		}
		for left, right := 0, len(queues)-1; left < right; left, right = left+1, right-1 {
			queues[left], queues[right] = queues[right], queues[left]
		}
	}
}

func TestPlanImportReviewOrderExternalLongChain(t *testing.T) {
	queues := make([]Queue, MaximumImportQueues)
	for index := range queues {
		queues[index] = importQueue(fmt.Sprintf("q%03d", index), fmt.Sprintf("q%03d", index+1))
	}
	plan, err := PlanImport(queues)
	if err != nil || plan.Ready || len(plan.Order) != 0 || len(plan.ReviewOrder) != len(queues) {
		t.Fatalf("plan=%+v err=%v", plan, err)
	}
	for index, entry := range plan.ReviewOrder {
		if entry != len(queues)-1-index {
			t.Fatal("prerequisite follows dependent")
		}
	}
	for index := range queues {
		queues[index].Spec.Replicas = 2
	}
	plan, err = PlanImport(queues)
	if err != nil || plan.ReviewOrder == nil || len(plan.ReviewOrder) != 0 {
		t.Fatal("invalid items must yield an empty, non-null review order")
	}
}
