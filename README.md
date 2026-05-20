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
```

Mini App methods:

- `Auth`
- `UserInfo`
- `CreateInvoice`
- `CheckInvoice`
- `CancelInvoice`
- `Refund`
