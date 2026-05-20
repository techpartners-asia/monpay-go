package monpay

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/techpartners-asia/monpay-go/utils"
)

var (
	MonpayGenerateQr = utils.API{
		Url:    "/rest/branch/qrpurchase/generate",
		Method: http.MethodPost,
	}
	MonpayCheckQr = utils.API{
		Url:    "/rest/branch/qrpurchase/check?uuid=",
		Method: http.MethodGet,
	}

	// MonpayMiniAppAuthToken [Mini App access token авах]
	MonpayMiniAppAuthToken = utils.API{
		Url:    "/oauth/token",
		Method: http.MethodPost,
	}
	// MonpayMiniAppUserInfo [Mini App хэрэглэгчийн мэдээлэл авах]
	MonpayMiniAppUserInfo = utils.API{
		Url:    "/api/oauth/userinfo",
		Method: http.MethodGet,
	}
	// MonpayMiniAppInvoiceCreate [Mini App нэхэмжлэх үүсгэх]
	MonpayMiniAppInvoiceCreate = utils.API{
		Url:    "/api/oauth/invoice",
		Method: http.MethodPost,
	}
	// MonpayMiniAppInvoiceCheck [Mini App нэхэмжлэх шалгах]
	MonpayMiniAppInvoiceCheck = utils.API{
		Url:    "/api/oauth/invoice/",
		Method: http.MethodGet,
	}
	// MonpayMiniAppInvoiceCancel [Mini App нэхэмжлэх цуцлах]
	MonpayMiniAppInvoiceCancel = utils.API{
		Url:    "/api/oauth/invoice/cancel",
		Method: http.MethodGet,
	}
	// MonpayMiniAppInvoiceRefund [Mini App нэхэмжлэх буцаах]
	MonpayMiniAppInvoiceRefund = utils.API{
		Url:    "/api/oauth/invoice/refund",
		Method: http.MethodGet,
	}

	MonpayDeeplinkCreate = MonpayMiniAppInvoiceCreate
	MonpayDeeplinkCheck  = MonpayMiniAppInvoiceCheck
)

func (m *monpay) httpRequestMonpay(body interface{}, api utils.API, urlExt string) (response []byte, err error) {
	requestBody, err := jsonBodyReader(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(api.Method, m.endpoint+api.Url+urlExt, requestBody)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(m.username, m.accoutnId)
	req.Header.Add("Content-Type", utils.HttpContent)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	response, err = io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return response, newAPIError("Monpay response error", res.StatusCode, response)
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

		formBody := url.Values{}
		formBody.Add("client_id", d.clientId)
		formBody.Add("client_secret", d.clientSecret)
		formBody.Add("grant_type", defaultGrantType(d.grantType))

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
	req, err := http.NewRequest(http.MethodPost, d.endpoint+MonpayMiniAppAuthToken.Url, strings.NewReader(formBody.Encode()))
	if err != nil {
		return authToken, err
	}

	req.Header.Add("Content-Type", utils.XForm)
	req.Header.Add("Accept", utils.HttpContent)

	res, err := d.client.Do(req)
	if err != nil {
		return authToken, err
	}
	defer res.Body.Close()

	resp, err := io.ReadAll(res.Body)
	if err != nil {
		return authToken, err
	}
	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return authToken, newAPIError("Monpay auth failed", res.StatusCode, resp)
	}
	if err = json.Unmarshal(resp, &authToken); err != nil {
		return authToken, err
	}
	authToken.obtainedAt = time.Now()
	return authToken, nil
}

func (d *deeplink) httpRequestDeeplink(body interface{}, result interface{}, api utils.API, ext string, accessToken string) error {
	token := strings.TrimSpace(accessToken)
	if token == "" {
		auth, err := d.getAccessToken()
		if err != nil {
			return err
		}
		token = auth.AccessToken
	}

	requestBody, err := jsonBodyReader(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(api.Method, d.endpoint+api.Url+ext, requestBody)
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", utils.HttpContent)
	req.Header.Add("Accept", utils.HttpContent)
	req.Header.Add("Authorization", bearerToken(token))

	res, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	response, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return newAPIError("Monpay response error", res.StatusCode, response)
	}
	if result == nil || len(response) == 0 {
		return nil
	}
	if err = json.Unmarshal(response, result); err != nil {
		return err
	}
	return monpayBusinessError(result, response)
}

func jsonBodyReader(body interface{}) (*bytes.Reader, error) {
	if body == nil {
		return bytes.NewReader(nil), nil
	}

	requestByte, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(requestByte), nil
}

func defaultGrantType(grantType string) string {
	if strings.TrimSpace(grantType) == "" {
		return "client_credentials"
	}
	return grantType
}

func bearerToken(token string) string {
	token = strings.TrimSpace(token)
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		return token
	}
	return "Bearer " + token
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
