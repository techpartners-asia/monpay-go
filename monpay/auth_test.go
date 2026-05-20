package monpay

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestMiniAppAuthAndUserInfo(t *testing.T) {
	var clientAuthCalls atomic.Int32
	var userAuthCalls atomic.Int32
	srv := newMiniAppMockServer(t, &clientAuthCalls, &userAuthCalls)
	defer srv.Close()

	client := NewDeeplink(
		srv.URL+"/",
		"client-id",
		"client-secret",
		"client_credentials",
		"https://app.example/webhook",
		"https://app.example/callback",
		WithClient(srv.Client()),
		WithSyncAuth(),
	)

	token, err := client.Auth(MiniAppAuthInput{Code: "auth-code"})
	if err != nil {
		t.Fatalf("auth failed: %v", err)
	}
	if token.AccessToken != "user-token" {
		t.Fatalf("unexpected user token: %s", token.AccessToken)
	}

	userInfo, err := client.UserInfo("")
	if err != nil {
		t.Fatalf("userinfo failed: %v", err)
	}
	if userInfo.Result.UserPhone.String() != "99112233" {
		t.Fatalf("unexpected user phone: %s", userInfo.Result.UserPhone.String())
	}

	if clientAuthCalls.Load() != 1 {
		t.Fatalf("expected cached client token to use one auth call, got %d", clientAuthCalls.Load())
	}
	if userAuthCalls.Load() != 1 {
		t.Fatalf("expected one user auth call, got %d", userAuthCalls.Load())
	}
}

func TestMiniAppServerDownReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close()

	client := NewDeeplink(
		srv.URL,
		"client-id",
		"client-secret",
		"client_credentials",
		"https://app.example/webhook",
		"https://app.example/callback",
		WithClient(srv.Client()),
	)

	_, err := client.CreateInvoice(MiniAppCreateInvoiceInput{
		Amount:      5000,
		Receiver:    "branch",
		InvoiceType: P2B,
		Description: "Demo",
	})
	if err == nil {
		t.Fatal("expected server down error, got nil")
	}
}

func newMiniAppMockServer(t *testing.T, clientAuthCalls *atomic.Int32, userAuthCalls *atomic.Int32) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/oauth/token":
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse form: %v", err)
			}
			if r.Form.Get("client_id") != "client-id" || r.Form.Get("client_secret") != "client-secret" {
				t.Fatalf("unexpected credentials: %s/%s", r.Form.Get("client_id"), r.Form.Get("client_secret"))
			}

			switch r.Form.Get("grant_type") {
			case "authorization_code":
				userAuthCalls.Add(1)
				if r.Form.Get("code") != "auth-code" {
					t.Fatalf("unexpected auth code: %s", r.Form.Get("code"))
				}
				if r.Form.Get("redirect_uri") != "https://app.example/callback" {
					t.Fatalf("unexpected redirect uri: %s", r.Form.Get("redirect_uri"))
				}
				_ = json.NewEncoder(w).Encode(AccessToken{
					AccessToken: "user-token",
					TokenType:   "Bearer",
					ExpiresIn:   3600,
					Scope:       "phone email name",
				})
			case "client_credentials":
				clientAuthCalls.Add(1)
				_ = json.NewEncoder(w).Encode(AccessToken{
					AccessToken: "client-token",
					TokenType:   "Bearer",
					ExpiresIn:   3600,
					Scope:       "invoice",
				})
			default:
				t.Fatalf("unexpected grant type: %s", r.Form.Get("grant_type"))
			}
		case "/api/oauth/userinfo":
			requireAuth(t, r, "Bearer user-token")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code":    "SUCCESS",
				"intCode": 0,
				"info":    "ok",
				"result": map[string]interface{}{
					"userId":        11,
					"userPhone":     99112233,
					"userEmail":     "user@example.com",
					"userFirstname": "Test",
					"userLastname":  "User",
				},
			})
		case "/api/oauth/invoice":
			requireAuth(t, r, "Bearer client-token")
			var req MiniAppCreateInvoiceRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode create invoice request: %v", err)
			}
			if req.RedirectUri != "https://app.example/callback" {
				t.Fatalf("unexpected redirect uri: %s", req.RedirectUri)
			}
			if req.ClientServiceUrl != "https://app.example/webhook" {
				t.Fatalf("unexpected client service url: %s", req.ClientServiceUrl)
			}
			writeInvoiceResponse(w, 42, "NEW")
		case "/api/oauth/invoice/42":
			requireAuth(t, r, "Bearer client-token")
			writeInvoiceResponse(w, 42, "PAID")
		case "/api/oauth/invoice/cancel":
			requireAuth(t, r, "Bearer client-token")
			if r.URL.Query().Get("invoiceId") != "42" {
				t.Fatalf("unexpected cancel invoiceId: %s", r.URL.RawQuery)
			}
			writeInvoiceResponse(w, 42, "CANCELLED")
		case "/api/oauth/invoice/refund":
			requireAuth(t, r, "Bearer client-token")
			if r.URL.Query().Get("invoiceId") != "42" {
				t.Fatalf("unexpected refund invoiceId: %s", r.URL.RawQuery)
			}
			writeInvoiceResponse(w, 42, "REFUNDED")
		default:
			http.NotFound(w, r)
		}
	}))
}

func requireAuth(t *testing.T, r *http.Request, expected string) {
	t.Helper()
	if got := r.Header.Get("Authorization"); got != expected {
		t.Fatalf("unexpected authorization header: got %q want %q", got, expected)
	}
}

func writeInvoiceResponse(w http.ResponseWriter, id int, status string) {
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    "SUCCESS",
		"intCode": 0,
		"info":    "ok",
		"result": map[string]interface{}{
			"id":          id,
			"receiver":    "branch",
			"amount":      5000,
			"userId":      11,
			"miniAppId":   99,
			"createDate":  "2026-05-19T09:00:00Z",
			"updateDate":  "2026-05-19T09:01:00Z",
			"status":      status,
			"description": "Demo",
			"txnId":       "txn-1",
			"statusInfo":  "ok",
			"redirectUri": "https://app.example/callback",
			"invoiceType": "P2B",
		},
	})
}
