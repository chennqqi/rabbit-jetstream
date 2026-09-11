package config

import "testing"

func TestLocalDemoBindBoundary(t *testing.T) {
	for _, tc := range []struct {
		address string
		allowed bool
	}{
		{"127.0.0.1:8223", true}, {"[::1]:8223", true}, {"127.0.0.2:8223", true},
		{":8223", false}, {"0.0.0.0:8223", false}, {"[::]:8223", false}, {"localhost:8223", false}, {"192.168.1.2:8223", false}, {"invalid", false},
	} {
		t.Run(tc.address, func(t *testing.T) {
			err := (Config{HTTPAddr: tc.address, LocalDemo: true}).ValidateHTTPAccess()
			if (err == nil) != tc.allowed {
				t.Fatalf("allowed=%v err=%v", tc.allowed, err)
			}
		})
	}
	t.Setenv("RJS_LOCAL_DEMO", "")
	if FromEnv().LocalDemo {
		t.Fatal("anonymous demo enabled by default")
	}
	t.Setenv("RJS_LOCAL_DEMO", "invalid")
	if FromEnv().LocalDemo {
		t.Fatal("invalid configuration enabled anonymous demo")
	}
}
