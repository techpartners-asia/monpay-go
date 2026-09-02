package monpay

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/techpartners-asia/monpay-go/utils"
)

var (
	// MonpayGenerateQr [QR төлбөр үүсгэх]
	// See: POST {endpoint}/rest/branch/qrpurchase/generate
	MonpayGenerateQr = utils.API{
		Url:    "/rest/branch/qrpurchase/generate",
		Method: http.MethodPost,
	}
	// MonpayCheckQr [QR төлбөр шалгах]
	// See: GET {endpoint}/rest/branch/qrpurchase/check?uuid={uuid}
	MonpayCheckQr = utils.API{
		Url:    "/rest/branch/qrpurchase/check?uuid=",
		Method: http.MethodGet,
	}
	// MonpayPushNotification [Хэрэглэгч рүү push notification илгээх]
	// See: GET {endpoint}/rest/admin/notification/push
	MonpayPushNotification = utils.API{
		Url:    "/rest/admin/notification/push",
		Method: http.MethodGet,
	}

	// MonpayMiniAppAuthToken [Mini App access token авах]
	// See: POST {endpoint}/oauth/token
	MonpayMiniAppAuthToken = utils.API{
		Url:    "/oauth/token",
		Method: http.MethodPost,
	}
	// MonpayMiniAppUserInfo [Mini App хэрэглэгчийн мэдээлэл авах]
	// See: GET {endpoint}/api/oauth/userinfo
	MonpayMiniAppUserInfo = utils.API{
		Url:    "/api/oauth/userinfo",
		Method: http.MethodGet,
	}
	// MonpayMiniAppInvoiceCreate [Mini App нэхэмжлэх үүсгэх]
	// See: POST {endpoint}/api/oauth/invoice
	MonpayMiniAppInvoiceCreate = utils.API{
		Url:    "/api/oauth/invoice",
		Method: http.MethodPost,
	}
	// MonpayMiniAppInvoiceCheck [Mini App нэхэмжлэх шалгах]
	// See: GET {endpoint}/api/oauth/invoice/{invoiceId}
	MonpayMiniAppInvoiceCheck = utils.API{
		Url:    "/api/oauth/invoice/",
		Method: http.MethodGet,
	}
	// MonpayMiniAppInvoiceCancel [Mini App нэхэмжлэх цуцлах]
	// See: GET {endpoint}/api/oauth/invoice/cancel?invoiceId={invoiceId}
	MonpayMiniAppInvoiceCancel = utils.API{
		Url:    "/api/oauth/invoice/cancel",
		Method: http.MethodGet,
	}
	// MonpayMiniAppRefund [Mini App transaction refund хийх]
	// See: POST {endpoint}/api/oauth/refund
	MonpayMiniAppRefund = utils.API{
		Url:    "/api/oauth/refund",
		Method: http.MethodPost,
	}
	// MonpayMiniAppInvoiceRefund [Deprecated: use MonpayMiniAppRefund]
	// See: GET {endpoint}/api/oauth/invoice/refund?invoiceId={invoiceId}
	MonpayMiniAppInvoiceRefund = utils.API{
		Url:    "/api/oauth/invoice/refund",
		Method: http.MethodGet,
	}

	// MonpayDebtInvoiceCreate [Debt/penalty нэхэмжлэх үүсгэх]
	// See: POST {endpoint}/api/monpay/invoice
	MonpayDebtInvoiceCreate = utils.API{
		Url:    "/api/monpay/invoice",
		Method: http.MethodPost,
	}
	// MonpayDebtInvoiceGet [Debt нэхэмжлэхийн мэдээлэл авах]
	// See: GET {endpoint}/api/monpay/invoice/{invoiceId}
	MonpayDebtInvoiceGet = utils.API{
		Url:    "/api/monpay/invoice/",
		Method: http.MethodGet,
	}
	// MonpayDebtInvoiceCancel [Debt нэхэмжлэх цуцлах]
	// See: PUT {endpoint}/api/monpay/invoice/{invoiceId}
	MonpayDebtInvoiceCancel = utils.API{
		Url:    "/api/monpay/invoice/",
		Method: http.MethodPut,
	}
	// MonpayRedirect [Monpay app дээр invoice төлбөрийн хэрэгсэл харуулах]
	// See: GET {endpoint}/invoice/{invoiceId}
	MonpayRedirect = utils.API{
		Url:    "/invoice/",
		Method: http.MethodGet,
	}

	MonpayDeeplinkCreate = MonpayMiniAppInvoiceCreate
	MonpayDeeplinkCheck  = MonpayMiniAppInvoiceCheck
)

func (m *monpay) httpRequestMonpay(body interface{}, api utils.API, urlExt string) (response []byte, err error) {
	req := m.client.R().
		SetHeader("Content-Type", utils.HttpContent).
		SetHeader("Accept", utils.HttpContent).
		SetBasicAuth(m.username, m.accoutnId).
		SetResponseBodyUnlimitedReads(true)
	if body != nil {
		req.SetBody(body)
	}

	res, err := req.Execute(api.Method, m.endpoint+api.Url+urlExt)
	if err != nil {
		return nil, err
	}

	response = res.Bytes()
	if res.IsError() {
		return response, newAPIError("Monpay response error", res.StatusCode(), response)
	}
	return response, nil
}

func (d *deeplink) getAccessToken() (AccessToken, error) {
	d.mu.RLock()
	if d.accessToken != nil && d.tokenValid(d.accessToken) {
		authToken := *d.accessToken
		d.mu.RUnlock()
		return authToken, nil
	}
	d.mu.RUnlock()

	value, err, _ := d.authGroup.Do("mini-app-auth", func() (interface{}, error) {
		d.mu.RLock()
		if d.accessToken != nil && d.tokenValid(d.accessToken) {
			authToken := *d.accessToken
			d.mu.RUnlock()
			return authToken, nil
		}
		d.mu.RUnlock()

		// The app/merchant token is always a client-credentials grant. It must NOT
		// be driven by the configurable grantType (that value belongs to the user
		// authorization_code flow in Auth); sending grant_type=authorization_code
		// here has no code and fails with {"error":"invalid_request",
		// "error_description":"code required"}.
		formBody := url.Values{}
		formBody.Add("client_id", d.clientId)
		formBody.Add("client_secret", d.clientSecret)
		formBody.Add("grant_type", clientCredentialsGrant(d.grantType))

		authToken, authErr := d.doTokenRequest(formBody)
		if authErr != nil {
			return AccessToken{}, authErr
		}

		d.mu.Lock()
		d.accessToken = &authToken
		d.mu.Unlock()
		return authToken, nil
	})
	if err != nil {
		return AccessToken{}, err
	}

	authToken, ok := value.(AccessToken)
	if !ok {
		return AccessToken{}, errors.New("monpay auth response has unexpected type")
	}
	return authToken, nil
}

func (d *deeplink) doTokenRequest(formBody url.Values) (AccessToken, error) {
	var authToken AccessToken
	res, err := d.client.R().
		SetHeader("Content-Type", utils.XForm).
		SetHeader("Accept", utils.HttpContent).
		SetFormDataFromValues(formBody).
		SetResult(&authToken).
		SetResponseBodyUnlimitedReads(true).
		Post(d.endpoint + MonpayMiniAppAuthToken.Url)
	if err != nil {
		return authToken, err
	}

	resp := res.Bytes()
	if res.IsError() {
		return authToken, newAPIError("Monpay auth failed", res.StatusCode(), resp)
	}
	if len(resp) > 0 {
		if err = json.Unmarshal(resp, &authToken); err != nil {
			return authToken, err
		}
	}
	if strings.TrimSpace(authToken.AccessToken) == "" {
		return authToken, errors.New("monpay auth response missing access token")
	}
	authToken.obtainedAt = time.Now()
	return authToken, nil
}

func (d *deeplink) httpRequestDeeplink(body interface{}, result interface{}, api utils.API, ext string, accessToken string) error {
	// A caller-supplied token is used as-is and never refreshed (it is not ours
	// to re-mint). An empty token is auto-fetched from the client-credentials
	// cache, and such a managed token is transparently refreshed once if the
	// server rejects it as expired/invalid.
	managed := strings.TrimSpace(accessToken) == ""

	for attempt := 0; attempt < 2; attempt++ {
		token := strings.TrimSpace(accessToken)
		if token == "" {
			auth, err := d.getAccessToken()
			if err != nil {
				return err
			}
			token = auth.AccessToken
		}

		req := d.client.R().
			SetHeader("Content-Type", utils.HttpContent).
			SetHeader("Accept", utils.HttpContent).
			SetAuthToken(restyAuthToken(token)).
			SetResponseBodyUnlimitedReads(true)
		if result != nil {
			req.SetResult(result)
		}
		if body != nil {
			req.SetBody(body)
		}

		res, err := req.Execute(api.Method, d.endpoint+api.Url+ext)
		if err != nil {
			return err
		}

		response := res.Bytes()

		// Recover transparently from a stale/revoked managed token: the server
		// answers 401 UNAUTHORIZED, so drop the cached token and retry once with a
		// freshly minted one. A 401 INSUFFICIENT_ACCESS (a permission gap, not a
		// bad token) is NOT retried — a new token for the same app fails the same
		// way — so it surfaces immediately.
		if managed && attempt == 0 && res.StatusCode() == http.StatusUnauthorized && isExpiredTokenResponse(response) {
			d.invalidateAccessToken()
			continue
		}

		if res.IsError() {
			return newAPIError("Monpay response error", res.StatusCode(), response)
		}
		if result == nil || len(response) == 0 {
			return nil
		}
		if err = json.Unmarshal(response, result); err != nil {
			return err
		}
		return monpayBusinessError(result, response)
	}

	return errors.New("monpay request failed after token refresh")
}

// isExpiredTokenResponse reports whether a 401 body signals an invalid/expired
// access token (UNAUTHORIZED / intCode 1) — which a token refresh can fix —
// rather than a permission gap (INSUFFICIENT_ACCESS / intCode 6), which it can't.
func isExpiredTokenResponse(body []byte) bool {
	var e struct {
		Code    string `json:"code"`
		IntCode int    `json:"intCode"`
	}
	if err := json.Unmarshal(body, &e); err != nil {
		return false
	}
	return strings.EqualFold(e.Code, "UNAUTHORIZED") || e.IntCode == 1
}

// invalidateAccessToken clears the cached client-credentials token so the next
// getAccessToken mints a fresh one.
func (d *deeplink) invalidateAccessToken() {
	d.mu.Lock()
	d.accessToken = nil
	d.mu.Unlock()
}

// clientCredentialsGrant resolves the grant_type for the app/merchant token
// fetch. It defaults to client_credentials and never allows authorization_code
// (which needs a user code and cannot mint the client token).
func clientCredentialsGrant(grantType string) string {
	grantType = strings.TrimSpace(grantType)
	if grantType == "" || strings.EqualFold(grantType, "authorization_code") {
		return "client_credentials"
	}
	return grantType
}

func restyAuthToken(token string) string {
	token = strings.TrimSpace(token)
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		return strings.TrimSpace(token[len("bearer "):])
	}
	return token
}

func newAPIError(message string, statusCode int, body []byte) error {
	apiErr := &APIError{
		Message:    message,
		StatusCode: statusCode,
		Body:       string(body),
	}
	_ = json.Unmarshal(body, apiErr)
	return apiErr
}

func monpayBusinessError(result interface{}, raw []byte) error {
	type responseCode struct {
		Code    string `json:"code"`
		IntCode int    `json:"intCode"`
		Info    string `json:"info"`
	}

	data, err := json.Marshal(result)
	if err != nil {
		data = raw
	}

	var response responseCode
	if err := json.Unmarshal(data, &response); err != nil {
		return nil
	}
	if response.IntCode == 0 {
		return nil
	}
	return &APIError{
		Message: "Monpay business error",
		Code:    response.Code,
		IntCode: response.IntCode,
		Info:    response.Info,
		Body:    string(raw),
	}
}
