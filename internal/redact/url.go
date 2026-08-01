package redact

import "net/url"

// URL removes user information from an absolute URL before it is logged or
// returned by a management interface. Non-URLs are returned unchanged.
func URL(value string) string {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return value
	}
	parsed.User = nil
	return parsed.String()
}
