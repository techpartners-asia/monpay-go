package monpay

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// An installed ExternalToken is sent as-is, and the SDK mints nothing of its
// own: the auth endpoint must never be reached.
func TestExternalTokenIsUsedVerbatimAndNeverMinted(t *testing.T) {
	var authCalls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "token") {
			atomic.AddInt32(&authCalls, 1)
			http.Error(w, "auth must not be called", http.StatusInternalServerError)
			return
		}
		requireAuth(t, r, "Bearer caller-token")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code": "", "intCode": 0, "info": "",
			"result": map[string]interface{}{"id": 1, "status": "NEW"},
		})
	}))
	defer srv.Close()

	client := newExternalTestClient(srv, ExternalToken{AccessToken: "caller-token"})

	if _, err := client.GetDebtInvoice(1); err != nil {
		t.Fatalf("call with external token failed: %v", err)
	}
	if got := atomic.LoadInt32(&authCalls); got != 0 {
		t.Fatalf("SDK minted a token behind the caller's back: %d auth calls", got)
	}
}

// A rejected ExternalToken surfaces as ErrUnauthorized rather than being
// silently replaced — the SDK does not own it, so it cannot re-mint it.
func TestRejectedExternalTokenIsErrUnauthorizedAndNotRetried(t *testing.T) {
	var calls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code": "UNAUTHORIZED", "intCode": 1, "info": "token expired",
		})
	}))
	defer srv.Close()

	client := newExternalTestClient(srv, ExternalToken{AccessToken: "stale"})

	_, err := client.GetDebtInvoice(1)
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
	// Exactly one attempt: the managed path's refresh-and-retry must not apply.
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected 1 request, got %d — external token was retried", got)
	}
}

// FetchToken performs one request and installs nothing.
func TestFetchTokenReturnsTokenAndInstallsNothing(t *testing.T) {
	var authCalls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&authCalls, 1)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "fresh", "token_type": "Bearer", "expires_in": 3600,
		})
	}))
	defer srv.Close()

	// Constructed in external mode so the managed background pre-warm does not
	// fire and the only auth request counted is FetchToken's own.
	client := newExternalTestClient(srv, ExternalToken{AccessToken: "placeholder"})

	token, err := client.FetchToken(context.Background())
	if err != nil {
		t.Fatalf("FetchToken failed: %v", err)
	}
	if token.AccessToken != "fresh" {
		t.Fatalf("unexpected token: %+v", token)
	}
	if token.ExpiresAt.IsZero() {
		t.Fatal("expires_in of 3600 should anchor an expiry")
	}
	if got := atomic.LoadInt32(&authCalls); got != 1 {
		t.Fatalf("expected exactly 1 auth request, got %d", got)
	}
	if installed := client.InstalledToken(); installed.AccessToken != "placeholder" {
		t.Fatalf("FetchToken must install nothing, but the token changed to %+v", installed)
	}
}

// Clearing the external token hands management back to the SDK.
func TestClearingExternalTokenRestoresManagedMode(t *testing.T) {
	var authCalls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "token") {
			atomic.AddInt32(&authCalls, 1)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token": "managed", "token_type": "Bearer", "expires_in": 3600,
			})
			return
		}
		requireAuth(t, r, "Bearer managed")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code": "", "intCode": 0, "info": "",
			"result": map[string]interface{}{"id": 1, "status": "NEW"},
		})
	}))
	defer srv.Close()

	client := newExternalTestClient(srv, ExternalToken{AccessToken: "caller-token"})
	client.UseExternalToken(ExternalToken{})

	if _, err := client.GetDebtInvoice(1); err != nil {
		t.Fatalf("call after clearing external token failed: %v", err)
	}
	if got := atomic.LoadInt32(&authCalls); got != 1 {
		t.Fatalf("expected the SDK to mint its own token once, got %d", got)
	}
}

// A no-expires_in response leaves ExpiresAt zero: the gateway said nothing, so
// the caller decides. Documented on ExternalToken.ExpiresAt.
func TestExternalTokenWithoutExpiresInHasZeroExpiry(t *testing.T) {
	token := externalTokenFrom(AccessToken{AccessToken: "x"}, time.Now())
	if !token.ExpiresAt.IsZero() {
		t.Fatalf("expected zero ExpiresAt, got %v", token.ExpiresAt)
	}
}

func newExternalTestClient(srv *httptest.Server, token ExternalToken) Deeplink {
	opts := []Option{WithClient(newTestRestyClient(srv))}
	if !token.IsZero() {
		opts = append(opts, WithExternalToken(token))
	}
	return NewDeeplink(
		srv.URL,
		"client-id",
		"client-secret",
		"client_credentials",
		"https://app.example/webhook",
		"https://app.example/callback",
		opts...,
	)
}
