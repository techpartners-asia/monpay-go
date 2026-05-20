package monpay

import (
	"sync/atomic"
	"testing"
)

func TestMiniAppInvoiceLifecycle(t *testing.T) {
	var clientAuthCalls atomic.Int32
	var userAuthCalls atomic.Int32
	srv := newMiniAppMockServer(t, &clientAuthCalls, &userAuthCalls)
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

	created, err := client.CreateInvoice(MiniAppCreateInvoiceInput{
		Amount:      5000,
		Receiver:    "branch",
		InvoiceType: P2B,
		Description: "Demo",
	})
	if err != nil {
		t.Fatalf("create invoice failed: %v", err)
	}
	if created.Result.ID != 42 || created.Result.Status != "NEW" {
		t.Fatalf("unexpected create invoice result: %+v", created.Result)
	}

	checked, err := client.CheckInvoice(42)
	if err != nil {
		t.Fatalf("check invoice failed: %v", err)
	}
	if checked.Result.Status != "PAID" {
		t.Fatalf("unexpected checked status: %s", checked.Result.Status)
	}

	cancelled, err := client.CancelInvoice(42)
	if err != nil {
		t.Fatalf("cancel invoice failed: %v", err)
	}
	if cancelled.Result.Status != "CANCELLED" {
		t.Fatalf("unexpected cancel status: %s", cancelled.Result.Status)
	}

	refunded, err := client.Refund(42)
	if err != nil {
		t.Fatalf("refund invoice failed: %v", err)
	}
	if refunded.Result.Status != "REFUNDED" {
		t.Fatalf("unexpected refund status: %s", refunded.Result.Status)
	}

	if clientAuthCalls.Load() != 1 {
		t.Fatalf("expected cached client token to use one auth call, got %d", clientAuthCalls.Load())
	}
	if userAuthCalls.Load() != 0 {
		t.Fatalf("expected no user auth calls, got %d", userAuthCalls.Load())
	}
}
