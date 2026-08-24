package main

import "testing"

func TestValidatePrefix(t *testing.T) {
	for _, test := range []struct {
		prefix string
		valid  bool
	}{
		{prefix: "RJSQ_sdk_perf_", valid: true},
		{prefix: "RJSQ_sdk_perf_123", valid: true},
		{prefix: "RJSQ_SDK_PERF_"},
		{prefix: "RJS_AUDIT_EVENTS"},
		{prefix: "KV_RJS_META"},
		{prefix: ""},
	} {
		if err := validatePrefix(test.prefix); (err == nil) != test.valid {
			t.Errorf("validatePrefix(%q) error = %v, valid=%t", test.prefix, err, test.valid)
		}
	}
}
