package monpay

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDebtInvoiceLifecycle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/monpay/invoice":
			requireAuth(t, r, "Bearer client-token")
			var req CreateDebtInvoiceRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode create debt invoice: %v", err)
			}
			if req.MiniAppInvoiceID != 49751 || req.ExternalInvoiceID != "12340" {
				t.Fatalf("unexpected create body: %+v", req)
			}
			if len(req.InvoiceItem) != 1 || req.InvoiceItem[0].ItemCode != "test-01" || req.InvoiceItem[0].UnitPrice != 5000 {
				t.Fatalf("unexpected invoice items: %+v", req.InvoiceItem)
			}
			if req.InvoiceCategory != InvoiceCategoryPenalty {
				t.Fatalf("unexpected category: %s", req.InvoiceCategory)
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code": "", "intCode": 0, "info": "",
				"result": map[string]interface{}{
					"reference": "REF-1", "requestedCount": 1, "successCount": 1, "failureCount": 0,
					"results": []map[string]interface{}{
						{
							"recipientType": "INDIVIDUAL", "recipientIdentifier": "99112233", "result": "SUCCESS",
							"invoice": map[string]interface{}{
								"id": 777, "invoiceNumber": "INV-777", "externalInvoiceId": "12340",
								"sourceInvoiceId": "49751", "category": "PENALTY", "status": "NEW",
								"totalAmount": 5000, "collectedAmount": 0,
							},
						},
					},
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/monpay/invoice/777":
			requireAuth(t, r, "Bearer client-token")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code": "", "intCode": 0, "info": "",
				"result": map[string]interface{}{
					"id": 777, "invoiceNumber": "INV-777", "category": "PENALTY",
					"type": "DEBT", "status": "NEW", "totalAmount": 5000, "collectedAmount": 0,
				},
			})
		case r.Method == http.MethodPut && r.URL.Path == "/api/monpay/invoice/777":
			requireAuth(t, r, "Bearer client-token")
			var req CancelDebtInvoiceRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode cancel debt invoice: %v", err)
			}
			if req.ReasonCode != "Cancel Request" {
				t.Fatalf("unexpected cancel body: %+v", req)
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code": "", "intCode": 0, "info": "",
				"result": map[string]interface{}{"id": 777, "status": "CANCELLED"},
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
		WithClient(newTestRestyClient(srv)),
		WithAccessToken(AccessToken{AccessToken: "client-token"}),
	)

	created, err := client.CreateDebtInvoice(CreateDebtInvoiceInput{
		MiniAppInvoiceID:  49751,
		ExternalInvoiceID: "12340",
		Items: []DebtInvoiceItemInput{
			{ItemCode: "test-01", Description: "PowerBank geesen tul nehemjlel uusgev", UnitPrice: 5000},
		},
		Category: InvoiceCategoryPenalty,
	})
	if err != nil {
		t.Fatalf("create debt invoice failed: %v", err)
	}
	if created.Result.SuccessCount != 1 || len(created.Result.Results) != 1 {
		t.Fatalf("unexpected create result: %+v", created.Result)
	}
	if created.Result.Results[0].Invoice.ID != 777 {
		t.Fatalf("unexpected invoice id: %d", created.Result.Results[0].Invoice.ID)
	}

	got, err := client.GetDebtInvoice(777)
	if err != nil {
		t.Fatalf("get debt invoice failed: %v", err)
	}
	if got.Result.ID != 777 || got.Result.Type != "DEBT" {
		t.Fatalf("unexpected get result: %+v", got.Result)
	}

	cancelled, err := client.CancelDebtInvoice(777, CancelDebtInvoiceInput{
		ReasonCode: "Cancel Request",
		ReasonText: "reasonText",
	})
	if err != nil {
		t.Fatalf("cancel debt invoice failed: %v", err)
	}
	if cancelled.Result.Status != "CANCELLED" {
		t.Fatalf("unexpected cancel status: %s", cancelled.Result.Status)
	}
}

func TestCreateDebtInvoiceRequiresItems(t *testing.T) {
	client := NewDeeplink(
		"https://example.com",
		"client-id",
		"client-secret",
		"client_credentials",
		"https://app.example/webhook",
		"https://app.example/callback",
		WithAccessToken(AccessToken{AccessToken: "client-token"}),
	)

	if _, err := client.CreateDebtInvoice(CreateDebtInvoiceInput{MiniAppInvoiceID: 1}); err == nil {
		t.Fatal("expected validation error for missing items")
	}
}

func TestGetDebtInvoiceRequiresID(t *testing.T) {
	client := NewDeeplink(
		"https://example.com",
		"client-id",
		"client-secret",
		"client_credentials",
		"https://app.example/webhook",
		"https://app.example/callback",
		WithAccessToken(AccessToken{AccessToken: "client-token"}),
	)

	if _, err := client.GetDebtInvoice(0); err == nil {
		t.Fatal("expected validation error for missing invoice id")
	}
}
