package api

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chennqqi/rabbit-jetstream/management/internal/identity"
)

type fakeAccountStore struct {
	createErr error
	updateErr error
	deleteErr error
}

func (s *fakeAccountStore) ListAccounts(context.Context) ([]identity.LocalAccountView, error) {
	return nil, nil
}
func (s *fakeAccountStore) CreateAccount(context.Context, identity.LocalAccountChange) (identity.LocalAccountView, error) {
	return identity.LocalAccountView{}, s.createErr
}
func (s *fakeAccountStore) UpdateAccount(context.Context, identity.LocalAccountChange) (identity.LocalAccountView, error) {
	return identity.LocalAccountView{}, s.updateErr
}
func (s *fakeAccountStore) DeleteAccount(_ context.Context, _ string) error { return s.deleteErr }

func accountStoreHandler(store identity.LocalAccountManager) http.Handler {
	return NewWithControllerAuth(&fakeBackend{}, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", "dev", nil, nil, AuthConfig{
		OperatorTokens: []string{"operator-token"},
		TenantIDs:      []string{"local"},
		LocalAccounts:  store,
	})
}

func platformAdminRequest(handler http.Handler, method, target, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer operator-token")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func TestAccountStoreErrorsMapToDistinctCodes(t *testing.T) {
	store := &fakeAccountStore{createErr: identity.ErrAccountExists}
	handler := accountStoreHandler(store)
	recorder := platformAdminRequest(handler, http.MethodPost, "/api/v1/access/accounts", `{"username":"alice","password":"a long enough password","memberships":[{"tenant":"local","role":"operator"}]}`)
	if recorder.Code != http.StatusConflict || !strings.Contains(recorder.Body.String(), `"code":"account_already_exists"`) {
		t.Fatalf("create exists: status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	store = &fakeAccountStore{createErr: identity.ErrAccountPolicy}
	handler = accountStoreHandler(store)
	recorder = platformAdminRequest(handler, http.MethodPost, "/api/v1/access/accounts", `{"username":"alice","password":"a long enough password","memberships":[{"tenant":"local","role":"operator"}]}`)
	if recorder.Code != http.StatusConflict || !strings.Contains(recorder.Body.String(), `"code":"account_policy_rejected"`) {
		t.Fatalf("create policy: status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	store = &fakeAccountStore{createErr: identity.ErrAccountValidation}
	handler = accountStoreHandler(store)
	recorder = platformAdminRequest(handler, http.MethodPost, "/api/v1/access/accounts", `{"username":"alice","password":"a long enough password","memberships":[{"tenant":"local","role":"operator"}]}`)
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), `"code":"invalid_account_request"`) {
		t.Fatalf("create validation: status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	store = &fakeAccountStore{updateErr: identity.ErrAccountNotFound}
	handler = accountStoreHandler(store)
	recorder = platformAdminRequest(handler, http.MethodPut, "/api/v1/access/accounts/ghost", `{"username":"ghost","memberships":[{"tenant":"local","role":"auditor"}]}`)
	if recorder.Code != http.StatusNotFound || !strings.Contains(recorder.Body.String(), `"code":"account_not_found"`) {
		t.Fatalf("update missing: status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	store = &fakeAccountStore{deleteErr: identity.ErrAccountStoreUnavailable}
	handler = accountStoreHandler(store)
	recorder = platformAdminRequest(handler, http.MethodDelete, "/api/v1/access/accounts/alice", "")
	if recorder.Code != http.StatusServiceUnavailable || !strings.Contains(recorder.Body.String(), `"code":"local_account_store_unavailable"`) {
		t.Fatalf("delete unavailable: status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	store = &fakeAccountStore{deleteErr: errors.New("disk on fire")}
	handler = accountStoreHandler(store)
	recorder = platformAdminRequest(handler, http.MethodDelete, "/api/v1/access/accounts/alice", "")
	if recorder.Code != http.StatusConflict || !strings.Contains(recorder.Body.String(), `"code":"account_delete_rejected"`) {
		t.Fatalf("delete unknown keeps the conflict bucket: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
