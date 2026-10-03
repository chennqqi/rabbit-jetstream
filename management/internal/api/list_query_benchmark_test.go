package api

import (
	"fmt"
	"net/http/httptest"
	"testing"
)

// The console scale target is 10k Queues / 100k Consumers served from a full
// backend enumeration (docs/capacity-planning.md). This benchmark pins the
// in-memory filter+sort cost of one list request at that scale so a contract
// change that regresses it surfaces in CI rather than in production.
func BenchmarkQueryList10k(b *testing.B) {
	items := make([]Item, 10000)
	for index := range items {
		items[index] = Item{Name: fmt.Sprintf("orders.service-%05d", index)}
	}
	request := httptest.NewRequest("GET", "/api/v1/queues?q=service-00&order=desc", nil)
	query, err := parseListQuery(request)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for range b.N {
		filtered := queryList(items, query, func(item Item) string { return item.Name })
		if len(filtered) != 1000 {
			b.Fatalf("unexpected match count %d", len(filtered))
		}
	}
}

type Item struct{ Name string }
