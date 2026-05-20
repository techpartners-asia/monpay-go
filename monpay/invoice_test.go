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

	refundTransaction, err := client.RefundTransaction(MiniAppRefundInput{
		InvoiceID:   42,
		Description: "Customer requested refund",
	})
	if err != nil {
		t.Fatalf("refund transaction by invoice id failed: %v", err)
	}
	if refundTransaction.Result.RefundTxnID != "RF202605200010" {
		t.Fatalf("unexpected refund transaction id: %s", refundTransaction.Result.RefundTxnID)
	}
	if refundTransaction.Result.Phone.String() != "99112233" {
		t.Fatalf("unexpected refund phone: %s", refundTransaction.Result.Phone.String())
	}

	refundByTxnNo, err := client.RefundTransaction(MiniAppRefundInput{
		TxnNo:       "TXN202605200001",
		Description: "Duplicate payment refund",
	})
	if err != nil {
		t.Fatalf("refund transaction by txn no failed: %v", err)
	}
	if refundByTxnNo.Result.TxnID != "TXN202605200001" {
		t.Fatalf("unexpected original transaction id: %s", refundByTxnNo.Result.TxnID)
	}

	if clientAuthCalls.Load() != 1 {
		t.Fatalf("expected cached client token to use one auth call, got %d", clientAuthCalls.Load())
	}
	if userAuthCalls.Load() != 0 {
		t.Fatalf("expected no user auth calls, got %d", userAuthCalls.Load())
	}
}

func TestMiniAppRefundTransactionRequiresInvoiceIDOrTxnNo(t *testing.T) {
	client := NewDeeplink(
		"https://example.com",
		"client-id",
		"client-secret",
		"client_credentials",
		"https://app.example/webhook",
		"https://app.example/callback",
		WithAccessToken(AccessToken{AccessToken: "client-token"}),
	)

	_, err := client.RefundTransaction(MiniAppRefundInput{Description: "missing target"})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
}
