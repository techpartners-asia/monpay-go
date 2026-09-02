package monpay

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
)

// A managed (auto-fetched) token that comes back 401 UNAUTHORIZED is refreshed
// and the request retried once, so a stale/revoked cached token recovers on its
// own instead of failing every call until it expires.
func TestDeeplinkRefreshesTokenOnUnauthorized(t *testing.T) {
	var authCalls, invoiceCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/oauth/token":
			n := authCalls.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token": "token-" + strconv.Itoa(int(n)),
				"token_type":   "Bearer",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/monpay/invoice/5":
			if invoiceCalls.Add(1) == 1 {
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"code": "UNAUTHORIZED", "intCode": 1, "info": "re-login",
				})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code": "", "intCode": 0, "info": "",
				"result": map[string]interface{}{"id": 5, "status": "NEW"},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := NewDeeplink(
		srv.URL, "client-id", "client-secret", "client_credentials",
		"https://app.example/webhook", "https://app.example/callback",
		WithClient(newTestRestyClient(srv)),
		WithSyncAuth(),
	)

	got, err := client.GetDebtInvoice(5)
	if err != nil {
		t.Fatalf("expected retry to succeed, got: %v", err)
	}
	if got.Result.ID != 5 {
		t.Fatalf("unexpected result after retry: %+v", got.Result)
	}
	if invoiceCalls.Load() != 2 {
		t.Fatalf("expected the request to be retried once (2 calls), got %d", invoiceCalls.Load())
	}
	if authCalls.Load() < 2 {
		t.Fatalf("expected the token to be refreshed (>=2 auth calls), got %d", authCalls.Load())
	}
}

// A 401 INSUFFICIENT_ACCESS is a permission gap, not a stale token, so it must
// surface immediately without a pointless refresh + retry.
func TestDeeplinkDoesNotRetryOnInsufficientAccess(t *testing.T) {
	var invoiceCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/oauth/token":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token": "token", "token_type": "Bearer",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/monpay/invoice/5":
			invoiceCalls.Add(1)
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code": "INSUFFICIENT_ACCESS", "intCode": 6, "info": "no access",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := NewDeeplink(
		srv.URL, "client-id", "client-secret", "client_credentials",
		"https://app.example/webhook", "https://app.example/callback",
		WithClient(newTestRestyClient(srv)),
		WithSyncAuth(),
	)

	if _, err := client.GetDebtInvoice(5); err == nil {
		t.Fatal("expected an error for INSUFFICIENT_ACCESS")
	}
	if invoiceCalls.Load() != 1 {
		t.Fatalf("expected no retry (1 call), got %d", invoiceCalls.Load())
	}
}
