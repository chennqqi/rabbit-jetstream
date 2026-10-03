package topology

import (
	"fmt"
	"sort"
)

const MaximumImportQueues = 100

type ImportProblem struct {
	Code       string `json:"code"`
	Dependency string `json:"dependency,omitempty"`
}

type ImportItem struct {
	Index        int             `json:"index"`
	Queue        string          `json:"queue"`
	Revision     string          `json:"revision,omitempty"`
	Dependencies []string        `json:"dependencies"`
	Problems     []ImportProblem `json:"problems"`
}

// ImportPlan is declaration-only evidence. Ready means all input declarations
// are valid and internally ordered, NOT that destination resources are ready or
// that any write is authorized. Order contains input indexes, never file paths.
type ImportPlan struct {
	Items []ImportItem `json:"items"`
	Order []int        `json:"order"`
	// ReviewOrder permits unresolved external prerequisites to reach explicit
	// destination preview. It never clears Problems or establishes readiness.
	ReviewOrder []int `json:"review_order"`
	Ready       bool  `json:"ready"`
}

func planningCopy(queue Queue) Queue {
	queue.Spec.Subjects = append([]string(nil), queue.Spec.Subjects...)
	queue.Spec.Bindings = append([]Binding(nil), queue.Spec.Bindings...)
	for index := range queue.Spec.Bindings {
		queue.Spec.Bindings[index].Keys = append([]string(nil), queue.Spec.Bindings[index].Keys...)
	}
	return queue
}

// PlanImport performs no I/O and never changes inputs. Problems are projected
// codes, not raw documents or potentially sensitive validation error strings.
// External dependencies stay unverified until a destination-aware stage checks
// them. Invalid/duplicate/cyclic prerequisites block their transitive dependents.
func PlanImport(queues []Queue) (ImportPlan, error) {
	if len(queues) == 0 || len(queues) > MaximumImportQueues {
		return ImportPlan{}, fmt.Errorf("import requires between 1 and %d Queues", MaximumImportQueues)
	}
	result := ImportPlan{Items: make([]ImportItem, len(queues)), Order: make([]int, 0), Ready: true}
	names := make(map[string][]int, len(queues))
	for index, queue := range queues {
		item := ImportItem{Index: index, Queue: queue.Metadata.Name, Dependencies: []string{}, Problems: []ImportProblem{}}
		plan, err := BuildPlan(planningCopy(queue))
		if err != nil {
			item.Problems = append(item.Problems, ImportProblem{Code: "invalid_declaration"})
		} else {
			item.Revision = plan.Revision
			item.Dependencies = plan.Dependencies
		}
		result.Items[index] = item
		names[item.Queue] = append(names[item.Queue], index)
	}
	for index := range result.Items {
		item := &result.Items[index]
		if len(names[item.Queue]) > 1 {
			item.Problems = append(item.Problems, ImportProblem{Code: "duplicate_queue"})
		}
		for _, dependency := range item.Dependencies {
			if len(names[dependency]) == 0 {
				item.Problems = append(item.Problems, ImportProblem{Code: "external_dependency_unverified", Dependency: dependency})
			}
		}
	}
	// DFS marks only actual cycle members; propagation below distinguishes
	// dependents of a cycle from the cycle itself.
	colors := make([]int, len(queues))
	stack := make([]int, 0, len(queues))
	cyclic := make([]bool, len(queues))
	var visit func(int)
	visit = func(index int) {
		if colors[index] == 2 {
			return
		}
		if colors[index] == 1 {
			for pos := len(stack) - 1; pos >= 0; pos-- {
				cyclic[stack[pos]] = true
				if stack[pos] == index {
					break
				}
			}
			return
		}
		colors[index] = 1
		stack = append(stack, index)
		for _, dependency := range result.Items[index].Dependencies {
			matches := names[dependency]
			if len(matches) == 1 && len(result.Items[matches[0]].Problems) == 0 {
				visit(matches[0])
			}
		}
		stack = stack[:len(stack)-1]
		colors[index] = 2
	}
	for index, item := range result.Items {
		if len(item.Problems) == 0 {
			visit(index)
		}
	}
	for index, cycle := range cyclic {
		if cycle {
			result.Items[index].Problems = append(result.Items[index].Problems, ImportProblem{Code: "dependency_cycle"})
		}
	}
	for changed := true; changed; {
		changed = false
		for index := range result.Items {
			item := &result.Items[index]
			if len(item.Problems) > 0 {
				continue
			}
			for _, dependency := range item.Dependencies {
				matches := names[dependency]
				if len(matches) != 1 || len(result.Items[matches[0]].Problems) > 0 {
					item.Problems = append(item.Problems, ImportProblem{Code: "blocked_dependency", Dependency: dependency})
					changed = true
					break
				}
			}
		}
	}
	result.Order = importOrdering(result.Items, names, false)
	result.ReviewOrder = importOrdering(result.Items, names, true)
	result.Ready = len(result.Order) == len(result.Items)
	return result, nil
}

func importOrdering(items []ImportItem, names map[string][]int, forReview bool) []int {
	order := make([]int, 0, len(items))
	ordered := make([]bool, len(items))
	for {
		ready := make([]int, 0)
		for index, item := range items {
			blocked := false
			for _, problem := range item.Problems {
				if !forReview || (problem.Code != "external_dependency_unverified" && problem.Code != "blocked_dependency") {
					blocked = true
				}
			}
			if ordered[index] || blocked {
				continue
			}
			eligible := true
			for _, dependency := range item.Dependencies {
				matches := names[dependency]
				if forReview && len(matches) == 0 {
					continue // Only preview can resolve this external prerequisite.
				}
				if len(matches) != 1 || !ordered[matches[0]] {
					eligible = false
					break
				}
			}
			if eligible {
				ready = append(ready, index)
			}
		}
		if len(ready) == 0 {
			break
		}
		sort.Slice(ready, func(i, j int) bool { return items[ready[i]].Queue < items[ready[j]].Queue })
		// Select one at a time so newly unlocked names participate in the same
		// deterministic lexical tie-break, independent of input ordering.
		index := ready[0]
		ordered[index] = true
		order = append(order, index)
	}
	return order
}
