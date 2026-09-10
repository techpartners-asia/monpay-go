# Monpay golang implementation

## Mini App

```go
client := monpay.NewDeeplink(
	"https://z-wallet.monpay.mn/v2",
	"client-id",
	"client-secret",
	"client_credentials",
	"https://your.domain/webhook",
	"https://your.domain/redirect",
	monpay.WithSyncAuth(),
)

token, err := client.Auth(monpay.MiniAppAuthInput{
	Code: "authorization-code",
})
if err != nil {
	return err
}

userInfo, err := client.UserInfo(token.AccessToken)
if err != nil {
	return err
}
_ = userInfo

invoice, err := client.CreateInvoice(monpay.MiniAppCreateInvoiceInput{
	Amount:      500000,
	Receiver:    "your_branch_username",
	InvoiceType: monpay.P2B,
	Description: "Demo App SMS",
})
if err != nil {
	return err
}

checked, err := client.CheckInvoice(invoice.Result.ID)
if err != nil {
	return err
}
_ = checked

if err := client.RedirectInvoice(invoice.Result.ID); err != nil {
	return err
}

refund, err := client.RefundTransaction(monpay.MiniAppRefundInput{
	InvoiceID:   invoice.Result.ID,
	Description: "Customer requested refund",
})
if err != nil {
	return err
}
_ = refund
```

Mini App methods:

- `Auth`
- `UserInfo`
- `CreateInvoice`
- `CheckInvoice`
- `RedirectInvoice`
- `CancelInvoice`
- `RefundTransaction`

## Push notification

```go
admin := monpay.New(
	"https://z-wallet.monpay.mn",
	"developer",
	"Password1",
	"",
)

notification, err := admin.SendPushNotification(monpay.MonpayPushNotificationInput{
	UserPhone: "94071041",
	Title:     "test",
	Text:      "test",
	ActionKey: "tino_charge",
	Action:    "tino_charge",
})
if err != nil {
	return err
}
_ = notification
```

## Caller-owned tokens (opt-in)

By default the client manages the merchant token itself: it mints one, caches
it, pre-warms it in the background at construction, and refreshes it once when
Monpay rejects it as expired. Nothing below changes that — it is opt-in.

Install an `ExternalToken` when the token lives outside this process (a shared
cache, or another service mints it) and the SDK must not mint a second one
behind your back:

```go
client := monpay.NewDeeplink(
	endpoint, clientID, clientSecret, "client_credentials", webhookURL, redirectURL,
	monpay.WithExternalToken(cached),
)

// or later, on an existing client
client.UseExternalToken(cached)
```

While a token is installed the client sends it verbatim and never re-mints or
refreshes it, and the background pre-warm at construction is skipped. A token
the SDK does not own is not its to replace — doing so would be invisible to
whoever does own it — so a rejection surfaces for you to act on:

```go
if errors.Is(err, monpay.ErrUnauthorized) {
	token, err := client.FetchToken(ctx) // one request, installs nothing
	if err != nil {
		return err
	}
	client.UseExternalToken(token)
	// retry — the rejected request was never processed, so this cannot
	// double-create an invoice
}
```

| Method | Behaviour |
|---|---|
| `FetchToken(ctx)` | One request to the token endpoint. Returns an `ExternalToken`; caches and installs nothing. |
| `UseExternalToken(t)` | Installs the token every subsequent merchant call carries. The zero value clears it and returns the client to managed mode. |
| `InstalledToken()` | The installed token, or the zero value in managed mode. |

`WithExternalToken` is not `WithAccessToken`: the latter seeds the SDK's own
managed cache and leaves refreshing to the SDK, the former takes refreshing
away from it entirely.

One caveat worth planning for: Monpay's client-credentials token often carries
no `expires_in` at all, so `ExternalToken.ExpiresAt` is the zero time — the
gateway told us nothing, and the SDK will not guess. Apply your own renewal
interval; the managed path assumes 30 minutes in this case, which is a
reasonable figure to mirror.
