package app

import (
	"context"
	"errors"
	"testing"

	"github.com/chennqqi/rabbit-jetstream/management/internal/jetstream"
	"github.com/chennqqi/rabbit-jetstream/management/internal/tenant"
)

func TestTenantRouterSelectsOnlyContextTenant(t *testing.T) {
	blue, green := &jetstream.Client{}, &jetstream.Client{}
	router := &tenantRouter{clients: map[string]*jetstream.Client{"blue": blue, "green": green}, defaultID: "blue"}
	if got, err := router.selected(tenant.WithContext(context.Background(), "green")); err != nil || got != green {
		t.Fatalf("selected green = %p, %v", got, err)
	}
	if got, err := router.selected(context.Background()); err != nil || got != blue {
		t.Fatalf("selected default = %p, %v", got, err)
	}
	if _, err := router.selected(tenant.WithContext(context.Background(), "missing")); !errors.Is(err, errTenantBackendMissing) {
		t.Fatalf("selected missing = %v", err)
	}
}
