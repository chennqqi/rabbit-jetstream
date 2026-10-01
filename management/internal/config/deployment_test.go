package config

import "testing"

func TestDeploymentProfile(t *testing.T) {
	for _, value := range []string{"", "unknown", "standalone", "cluster", "automatic", "CLUSTER"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("RJS_DEPLOYMENT_PROFILE", value)
			t.Setenv("RJS_LOCAL_DEMO", "false")
			cfg := FromEnv()
			valid := value == "" || value == "unknown" || value == "standalone" || value == "cluster"
			if (cfg.ValidateDeploymentProfile() == nil) != valid {
				t.Fatalf("profile=%q", cfg.DeploymentProfile)
			}
			if value == "" && cfg.DeploymentProfile != "unknown" {
				t.Fatal("deployment intent inferred")
			}
		})
	}
}
