package monpay

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

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

	MonpayDeeplinkCreate = utils.API{
		Url:    "/api/oauth/invoice",
		Method: http.MethodPost,
	}
	MonpayDeeplinkCheck = utils.API{
		Url:    "/api/oauth/invoice/",
		Method: http.MethodGet,
	}
)

func (m *monpay) httpRequestMonpay(body interface{}, api utils.API, urlExt string) (response []byte, err error) {
	var requestByte []byte
	var requestBody *bytes.Reader
	if body == nil {
		requestBody = bytes.NewReader(nil)
	} else {
		requestByte, _ = json.Marshal(body)
		requestBody = bytes.NewReader(requestByte)
	}

	req, _ := http.NewRequest(api.Method, m.endpoint+api.Url+urlExt, requestBody)
	req.SetBasicAuth(m.username, m.accoutnId)
	req.Header.Add("Content-Type", utils.HttpContent)

	res, err := http.DefaultClient.Do(req)
	response, _ = io.ReadAll(res.Body)
	if res.StatusCode != 200 {
		err = errors.New(string(response))
		fmt.Print(err.Error())
		return
	}
	defer res.Body.Close()
	return
}

func (d *deeplink) getAccessToken() (authToken AccessToken, err error) {
	if d.accessToken != nil {
		return *d.accessToken, nil
	}
	formBody := url.Values{}
	formBody.Add("client_id", d.clientId)
	formBody.Add("client_secret", d.clientSecret)
	formBody.Add("grant_type", d.grantType)
	req, _ := http.NewRequest(http.MethodPost, d.endpoint+"/oauth/token", strings.NewReader(formBody.Encode()))
	req.Header.Add("Content-Type", utils.XForm)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer res.Body.Close()
	resp, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 {
		err = errors.New(string(resp))
		fmt.Print(err.Error())
		return
	}
	err = json.Unmarshal(resp, &authToken)
	if err != nil {
		return
	}
	return
}

func (d *deeplink) httpRequestDeeplink(body interface{}, api utils.API, ext string) (response []byte, err error) {
	auth, err := d.getAccessToken()
	if err != nil {
		return nil, err
	}
	d.accessToken = &auth

	var requestByte []byte
	var requestBody *bytes.Reader
	if body == nil {
		requestBody = bytes.NewReader(nil)
	} else {
		requestByte, _ = json.Marshal(body)
		requestBody = bytes.NewReader(requestByte)
	}

	req, _ := http.NewRequest(api.Method, d.endpoint+api.Url+ext, requestBody)
	req.Header.Add("Content-Type", utils.HttpContent)
	req.Header.Add("Accept", utils.HttpContent)
	req.Header.Add("Authorization", "Bearer "+d.accessToken.AccessToken)
	res, err := http.DefaultClient.Do(req)
	response, _ = io.ReadAll(res.Body)
	if res.StatusCode != 200 {
		err = errors.New(string(response))
		fmt.Print(err.Error())
		return
	}
	defer res.Body.Close()
	return
}
