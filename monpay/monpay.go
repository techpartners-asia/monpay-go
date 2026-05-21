package monpay

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/schema"
	"github.com/techpartners-asia/monpay-go/utils"
	"golang.org/x/sync/singleflight"
	"resty.dev/v3"
)

type monpay struct {
	endpoint    string
	username    string
	accoutnId   string
	callbackurl string
	client      *resty.Client
}

type Monpay interface {
	GenerateQr(input MonpayQrInput) (MonpayQrResponse, error)
	CheckQr(uuid string) (MonpayCheckResponse, error)
	SendPushNotification(input MonpayPushNotificationInput) (MonpayPushNotificationResponse, error)
	CallbackParser(url *url.URL) MonpayCallback
}

func New(endpoint, username, accountId, callback string) Monpay {
	return &monpay{
		endpoint:    strings.TrimRight(endpoint, "/"),
		username:    username,
		accoutnId:   accountId,
		callbackurl: callback,
		client:      newRestyClient(),
	}
}

func (m *monpay) GenerateQr(input MonpayQrInput) (response MonpayQrResponse, err error) {
	invoice := MonpayQrRequest{
		Amount:       input.Amount,
		GenerateUUID: true,
		CallbackUrl:  m.callbackurl,
	}
	res, err := m.httpRequestMonpay(invoice, MonpayGenerateQr, "")
	if err != nil {
		return
	}
	if err = json.Unmarshal(res, &response); err != nil {
		return MonpayQrResponse{}, err
	}
	if response.Code != 0 {
		switch response.Code {
		case 5:
			err = errors.New("хүсэлт буруу")
		case 1:
			err = errors.New("unauthorized")
		case 999:
			err = errors.New("дотоод алдаа")
		default:
			err = errors.New("unknown error")
		}
	}
	return
}

func (m *monpay) CheckQr(uuid string) (response MonpayCheckResponse, err error) {
	res, err := m.httpRequestMonpay(nil, MonpayCheckQr, uuid)
	if err != nil {
		return
	}
	if err = json.Unmarshal(res, &response); err != nil {
		return MonpayCheckResponse{}, err
	}
	if response.Code != 0 {
		switch response.Code {
		case 5:
			err = errors.New("хүсэлт буруу")
		case 23:
			err = errors.New("QR not scanned")
		case 1:
			err = errors.New("unauthorized")
		case 999:
			err = errors.New("дотоод алдаа")
		default:
			err = errors.New("unknown error")
		}
	}
	return
}

// SendPushNotification [Хэрэглэгч рүү push notification илгээх]
func (m *monpay) SendPushNotification(input MonpayPushNotificationInput) (MonpayPushNotificationResponse, error) {
	query := url.Values{}
	query.Set("userPhone", input.UserPhone)
	query.Set("title", input.Title)
	query.Set("text", input.Text)
	query.Set("actionKey", input.ActionKey)
	query.Set("action", input.Action)

	var response MonpayPushNotificationResponse
	res, err := m.httpRequestMonpay(nil, MonpayPushNotification, "?"+query.Encode())
	if err != nil {
		return MonpayPushNotificationResponse{}, err
	}
	if err = json.Unmarshal(res, &response); err != nil {
		return MonpayPushNotificationResponse{}, err
	}
	if response.IntCode != 0 {
		return MonpayPushNotificationResponse{}, &APIError{
			Message: "Monpay push notification error",
			Code:    response.Code,
			IntCode: response.IntCode,
			Info:    response.Info,
			Body:    string(res),
		}
	}
	return response, nil
}

var decoder = schema.NewDecoder()

func (m *monpay) CallbackParser(url *url.URL) (response MonpayCallback) {
	err := decoder.Decode(&response, url.Query())
	if err != nil {
		log.Println("Error in GET parameters : ", err)
	} else {
		log.Println("GET parameters : ", response)
	}
	return
}

type deeplink struct {
	endpoint     string
	webhookUrl   string
	redirectUrl  string
	clientId     string
	clientSecret string
	grantType    string
	syncAuth     bool
	accessToken  *AccessToken
	userToken    *AccessToken
	mu           sync.RWMutex
	authGroup    singleflight.Group
	client       *resty.Client
}

type AccessToken struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int64  `json:"expires_in,omitempty"`
	Scope        string `json:"scope,omitempty"`
	obtainedAt   time.Time
}

type Deeplink interface {
	// Auth [Authorization code ашиглан хэрэглэгчийн access token авах]
	Auth(input MiniAppAuthInput) (AccessToken, error)

	// UserInfo [OAuth-оор нэвтэрсэн хэрэглэгчийн мэдээлэл авах]
	UserInfo(accessToken string) (response MiniAppUserInfoResponse, err error)

	// CreateInvoice [Mini App нэхэмжлэх үүсгэх]
	CreateInvoice(input MiniAppCreateInvoiceInput) (response MiniAppInvoiceResponse, err error)

	// CancelInvoice [Mini App нэхэмжлэх цуцлах]
	CancelInvoice(invoiceID int) (response MiniAppInvoiceResponse, err error)

	// Refund [Mini App нэхэмжлэх буцаах]
	//
	// Deprecated: use RefundTransaction.
	Refund(invoiceID int) (response MiniAppInvoiceResponse, err error)

	// RefundTransaction [Mini App-аар хийгдсэн transaction refund хийх]
	RefundTransaction(input MiniAppRefundInput) (response MiniAppRefundResponse, err error)

	CreateDeeplink(amount float64, invoiceType InvoiceType, branchUsername, desc, invoiceId string) (response DeeplinkCreateResponse, err error)
	CheckInvoice(invoiceID int) (response MiniAppInvoiceResponse, err error)
	CallbackParser(url *url.URL) (response DeeplinkCallback)
}

// Option defines an option for Mini App initialization.
type Option func(*deeplink)

// WithClient [Custom resty.Client ашиглах]
func WithClient(client *resty.Client) Option {
	return func(d *deeplink) {
		if client != nil {
			d.client = client
		}
	}
}

// WithSyncAuth [Эхлүүлэхдээ client credentials auth дуустал хүлээх]
func WithSyncAuth() Option {
	return func(d *deeplink) {
		d.syncAuth = true
	}
}

// WithAccessToken [Client credentials token-ийг гаднаас өгөх]
func WithAccessToken(token AccessToken) Option {
	return func(d *deeplink) {
		if strings.TrimSpace(token.AccessToken) == "" {
			return
		}
		token.obtainedAt = time.Now()
		d.accessToken = &token
	}
}

// NewDeeplink [Monpay Mini App SDK-ийг шинээр үүсгэх]
func NewDeeplink(endpoint, id, secret, grantType, webhookUrl, redirectUrl string, options ...Option) Deeplink {
	d := &deeplink{
		endpoint:     strings.TrimRight(endpoint, "/"),
		clientId:     id,
		clientSecret: secret,
		grantType:    grantType,
		webhookUrl:   webhookUrl,
		redirectUrl:  redirectUrl,
		client:       newRestyClient(),
	}

	for _, opt := range options {
		opt(d)
	}

	if d.syncAuth {
		for i := 0; i < 3; i++ {
			if _, err := d.getAccessToken(); err == nil {
				break
			}
			if i < 2 {
				time.Sleep(time.Second)
			}
		}
	} else {
		go d.getAccessToken() //nolint:errcheck
	}

	return d
}

// Auth [Authorization code ашиглан хэрэглэгчийн access token авах]
func (d *deeplink) Auth(input MiniAppAuthInput) (AccessToken, error) {
	grantType := strings.TrimSpace(input.GrantType)
	if grantType == "" {
		grantType = "authorization_code"
	}

	redirectURI := strings.TrimSpace(input.RedirectURI)
	if redirectURI == "" {
		redirectURI = d.redirectUrl
	}
	if strings.EqualFold(grantType, "authorization_code") && strings.TrimSpace(input.Code) == "" {
		return AccessToken{}, errors.New("monpay auth code is required")
	}

	formBody := url.Values{}
	formBody.Add("redirect_uri", redirectURI)
	formBody.Add("client_id", d.clientId)
	formBody.Add("client_secret", d.clientSecret)
	formBody.Add("code", input.Code)
	formBody.Add("grant_type", grantType)

	authToken, err := d.doTokenRequest(formBody)
	if err != nil {
		return AccessToken{}, err
	}

	d.mu.Lock()
	d.userToken = &authToken
	d.mu.Unlock()

	return authToken, nil
}

// UserInfo [OAuth-оор нэвтэрсэн хэрэглэгчийн мэдээлэл авах]
func (d *deeplink) UserInfo(accessToken string) (response MiniAppUserInfoResponse, err error) {
	token := strings.TrimSpace(accessToken)
	if token == "" {
		d.mu.RLock()
		if d.userToken != nil && d.tokenValid(d.userToken) {
			token = d.userToken.AccessToken
		}
		d.mu.RUnlock()
	}
	if token == "" {
		return MiniAppUserInfoResponse{}, errors.New("monpay user access token is required")
	}

	err = d.httpRequestDeeplink(nil, &response, MonpayMiniAppUserInfo, "", token)
	if err != nil {
		return MiniAppUserInfoResponse{}, err
	}
	return response, nil
}

func (d *deeplink) CreateDeeplink(amount float64, invoiceType InvoiceType, branchUsername, desc, invoiceId string) (response DeeplinkCreateResponse, err error) {
	return d.CreateInvoice(MiniAppCreateInvoiceInput{
		RedirectUri:      d.redirectUrl + "/" + invoiceId,
		ClientServiceUrl: d.webhookUrl + "/" + invoiceId,
		Amount:           amount,
		Receiver:         branchUsername,
		InvoiceType:      invoiceType,
		Description:      desc,
	})
}

// CreateInvoice [Mini App нэхэмжлэх үүсгэх]
func (d *deeplink) CreateInvoice(input MiniAppCreateInvoiceInput) (response MiniAppInvoiceResponse, err error) {
	body := MiniAppCreateInvoiceRequest{
		RedirectUri:      firstNonEmpty(input.RedirectUri, d.redirectUrl),
		ClientServiceUrl: firstNonEmpty(input.ClientServiceUrl, d.webhookUrl),
		Amount:           input.Amount,
		Receiver:         input.Receiver,
		InvoiceType:      input.InvoiceType,
		Description:      input.Description,
	}
	err = d.httpRequestDeeplink(body, &response, MonpayMiniAppInvoiceCreate, "", input.AccessToken)
	if err != nil {
		return MiniAppInvoiceResponse{}, err
	}
	return response, nil
}

// CheckInvoice [Mini App нэхэмжлэх шалгах]
func (d *deeplink) CheckInvoice(invoiceID int) (response MiniAppInvoiceResponse, err error) {
	if invoiceID <= 0 {
		return MiniAppInvoiceResponse{}, errors.New("monpay invoice id is required")
	}
	err = d.httpRequestDeeplink(nil, &response, MonpayMiniAppInvoiceCheck, fmt.Sprintf("%d", invoiceID), "")
	if err != nil {
		return MiniAppInvoiceResponse{}, err
	}
	return response, nil
}

// CancelInvoice [Mini App нэхэмжлэх цуцлах]
func (d *deeplink) CancelInvoice(invoiceID int) (response MiniAppInvoiceResponse, err error) {
	return d.invoiceAction(invoiceID, MonpayMiniAppInvoiceCancel)
}

// Refund [Mini App нэхэмжлэх буцаах]
//
// Deprecated: use RefundTransaction.
func (d *deeplink) Refund(invoiceID int) (response MiniAppInvoiceResponse, err error) {
	return d.invoiceAction(invoiceID, MonpayMiniAppInvoiceRefund)
}

// RefundTransaction [Mini App-аар хийгдсэн transaction refund хийх]
func (d *deeplink) RefundTransaction(input MiniAppRefundInput) (response MiniAppRefundResponse, err error) {
	if input.InvoiceID <= 0 && strings.TrimSpace(input.TxnNo) == "" {
		return MiniAppRefundResponse{}, errors.New("monpay refund invoice id or transaction no is required")
	}

	body := MiniAppRefundRequest{
		InvoiceID:   input.InvoiceID,
		TxnNo:       input.TxnNo,
		Description: input.Description,
	}
	err = d.httpRequestDeeplink(body, &response, MonpayMiniAppRefund, "", input.AccessToken)
	if err != nil {
		return MiniAppRefundResponse{}, err
	}
	return response, nil
}

func (d *deeplink) invoiceAction(invoiceID int, api utils.API) (MiniAppInvoiceResponse, error) {
	if invoiceID <= 0 {
		return MiniAppInvoiceResponse{}, errors.New("monpay invoice id is required")
	}

	query := url.Values{}
	query.Set("invoiceId", strconv.Itoa(invoiceID))

	var response MiniAppInvoiceResponse
	err := d.httpRequestDeeplink(nil, &response, api, "?"+query.Encode(), "")
	if err != nil {
		return MiniAppInvoiceResponse{}, err
	}
	return response, nil
}

func (d *deeplink) CallbackParser(url *url.URL) (response DeeplinkCallback) {
	err := decoder.Decode(&response, url.Query())
	if err != nil {
		log.Println("Error in GET parameters : ", err)
	} else {
		log.Println("GET parameters : ", response)
	}
	return
}

func (d *deeplink) tokenValid(token *AccessToken) bool {
	if token == nil || strings.TrimSpace(token.AccessToken) == "" {
		return false
	}
	if token.ExpiresIn <= 0 {
		return true
	}
	obtainedAt := token.obtainedAt
	if obtainedAt.IsZero() {
		return false
	}
	refreshAt := obtainedAt.Add(time.Duration(token.ExpiresIn) * time.Second).Add(-1 * time.Minute)
	return time.Now().Before(refreshAt)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func newRestyClient() *resty.Client {
	return resty.New().
		SetTransport(newTransport()).
		SetTimeout(60 * time.Second)
}

// newTransport creates an http.Transport with sensible defaults.
func newTransport() *http.Transport {
	return &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		MaxConnsPerHost:       20,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		ForceAttemptHTTP2:     true,
	}
}
