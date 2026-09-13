package api

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
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

func pagination(r *http.Request) (int, int, error) {
	offset, err := queryInt(r, "offset", 0)
	if err != nil || offset < 0 {
		return 0, 0, errors.New("offset must be a non-negative integer")
	}
	limit, err := queryInt(r, "limit", 50)
	if err != nil || limit < 1 || limit > 200 {
		return 0, 0, errors.New("limit must be an integer between 1 and 200")
	}
	return offset, limit, nil
}

func queryInt(r *http.Request, key string, fallback int) (int, error) {
	value := r.URL.Query().Get(key)
	if value == "" {
		return fallback, nil
	}
	return strconv.Atoi(value)
}

func page[T any](items []T, offset, limit int) map[string]any {
	total := len(items)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return map[string]any{"items": items[offset:end], "total": total, "offset": offset, "limit": limit}
}
