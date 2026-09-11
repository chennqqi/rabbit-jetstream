package api

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

type listQuery struct {
	search     string
	descending bool
}

func parseListQuery(r *http.Request) (listQuery, error) {
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return listQuery{}, fmt.Errorf("malformed query encoding")
	}
	for key, entries := range values {
		if len(entries) != 1 || (key != "q" && key != "sort" && key != "order" && key != "offset" && key != "limit") {
			return listQuery{}, fmt.Errorf("unsupported or repeated query parameter %s", key)
		}
	}
	search := strings.ToLower(strings.TrimSpace(values.Get("q")))
	if len(search) > 256 {
		return listQuery{}, fmt.Errorf("q exceeds 256 normalized UTF-8 bytes")
	}
	if field := values.Get("sort"); field != "" && field != "name" {
		return listQuery{}, fmt.Errorf("sort must be name")
	}
	order := values.Get("order")
	if order != "" && order != "asc" && order != "desc" {
		return listQuery{}, fmt.Errorf("order must be asc or desc")
	}
	return listQuery{search: search, descending: order == "desc"}, nil
}

// Apply the query to the full successful enumeration, never a loaded page.
// Copy the slice so the response cannot reorder backend-owned state.
func queryList[T any](items []T, query listQuery, name func(T) string) []T {
	filtered := make([]T, 0, len(items))
	for _, item := range items {
		if strings.Contains(strings.ToLower(name(item)), query.search) {
			filtered = append(filtered, item)
		}
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		if query.descending {
			return name(filtered[i]) > name(filtered[j])
		}
		return name(filtered[i]) < name(filtered[j])
	})
	return filtered
}
