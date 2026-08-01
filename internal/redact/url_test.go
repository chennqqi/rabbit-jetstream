package redact

import "testing"

func TestURL(t *testing.T) {
	for _, test := range []struct{ input, want string }{
		{"nats://user:secret@nats-1:4222", "nats://nats-1:4222"},
		{"https://token@example.test/path?q=1", "https://example.test/path?q=1"},
		{"plain value", "plain value"},
	} {
		if got := URL(test.input); got != test.want {
			t.Fatalf("URL(%q)=%q want=%q", test.input, got, test.want)
		}
	}
}
