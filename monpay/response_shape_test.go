package monpay

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMiniAppUserInfoAcceptsNumericPhone(t *testing.T) {
	var response MiniAppUserInfoResponse
	err := json.Unmarshal([]byte(`{
		"code": "SUCCESS",
		"intCode": 0,
		"info": "ok",
		"result": {
			"userId": 11,
			"userPhone": 99112233,
			"userEmail": "user@example.com",
			"userFirstname": "Test",
			"userLastname": "User"
		}
	}`), &response)
	if err != nil {
		t.Fatalf("failed to unmarshal user info: %v", err)
	}
	if response.Result.UserPhone.String() != "99112233" {
		t.Fatalf("unexpected user phone: %s", response.Result.UserPhone.String())
	}
}

func TestMiniAppBusinessErrorReturned(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/oauth/token":
			_ = json.NewEncoder(w).Encode(AccessToken{
				AccessToken: "client-token",
				TokenType:   "Bearer",
				ExpiresIn:   3600,
			})
		case "/api/oauth/invoice":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code":    "BAD_REQUEST",
				"intCode": 5,
				"info":    "bad request",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := NewDeeplink(
		srv.URL,
		"client-id",
		"client-secret",
		"client_credentials",
		"https://app.example/webhook",
		"https://app.example/callback",
		WithClient(srv.Client()),
		WithSyncAuth(),
	)

	_, err := client.CreateInvoice(MiniAppCreateInvoiceInput{
		Amount:      5000,
		Receiver:    "branch",
		InvoiceType: P2B,
		Description: "Demo",
	})
	if err == nil {
		t.Fatal("expected business error, got nil")
	}
	if !strings.Contains(err.Error(), "intCode=5") {
		t.Fatalf("expected intCode in error, got %v", err)
	}
}
