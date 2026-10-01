package api

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/chennqqi/rabbit-jetstream/management/internal/jetstream"
)

func parseStreamConsumerQuery(r *http.Request) (listQuery, string, error) {
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return listQuery{}, "", fmt.Errorf("malformed query encoding")
	}
	mode := values.Get("mode")
	if len(values["mode"]) > 1 || (mode != "" && mode != "pull" && mode != "push") {
		return listQuery{}, "", fmt.Errorf("mode must be pull or push and cannot be repeated")
	}
	values.Del("mode")
	// Reuse strict list validation without mutating the caller's request URL.
	copyRequest, copyURL := *r, *r.URL
	copyURL.RawQuery = values.Encode()
	copyRequest.URL = &copyURL
	query, err := parseListQuery(&copyRequest)
	return query, mode, err
}

// Search effective filter subjects as well as names before sorting/pagination.
// Sorting compares names only, never subject text or backend slice position.
func queryStreamConsumers(items []jetstream.Consumer, query listQuery, mode string) []jetstream.Consumer {
	filtered := make([]jetstream.Consumer, 0, len(items))
	for _, item := range items {
		if mode != "" && item.Mode != mode {
			continue
		}
		match := strings.Contains(strings.ToLower(item.Name), query.search)
		subjects := item.FilterSubjects
		if len(subjects) == 0 && item.FilterSubject != "" {
			subjects = []string{item.FilterSubject}
		}
		for _, subject := range subjects {
			match = match || strings.Contains(strings.ToLower(subject), query.search)
		}
		if match {
			filtered = append(filtered, item)
		}
	}
	return queryList(filtered, listQuery{descending: query.descending}, func(item jetstream.Consumer) string { return item.Name })
}
