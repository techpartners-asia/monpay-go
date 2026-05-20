package monpay

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type InvoiceType string

const (
	P2P InvoiceType = "P2P" // Хэрэглэгчээс хэрэглэгч
	P2B InvoiceType = "P2B" // Хэрэглэгчээс мерчант
	B2B InvoiceType = "B2B" // person to MF & MF to organization
)

type Bank string

const (
	BankKhan        Bank = "KHAN"
	BankTDB         Bank = "TDB"
	BankGolomt      Bank = "GOLOMT"
	BankState       Bank = "STATE"
	BankUlaanbaatar Bank = "ULAANBAATAR"
	BankXac         Bank = "XAC"
	BankCapitron    Bank = "CAPITRON"
	BankArig        Bank = "ARIG"
	BankChinggis    Bank = "CHINGGIS"
	BankBogd        Bank = "BOGD"
	BankCredit      Bank = "CREDIT"
	BankHugjil      Bank = "HUGJIL"
	BankTuriinsan   Bank = "TURIIN_SAN"
)

type FlexibleString string

func (s *FlexibleString) UnmarshalJSON(data []byte) error {
	raw := strings.TrimSpace(string(data))
	if raw == "" || raw == "null" {
		*s = ""
		return nil
	}

	var value string
	if err := json.Unmarshal(data, &value); err == nil {
		*s = FlexibleString(value)
		return nil
	}

	var number json.Number
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&number); err == nil {
		*s = FlexibleString(number.String())
		return nil
	}

	return fmt.Errorf("monpay: cannot unmarshal %s into FlexibleString", raw)
}

func (s FlexibleString) String() string {
	return string(s)
}

type APIError struct {
	Message    string `json:"-"`
	StatusCode int    `json:"-"`
	Code       string `json:"code,omitempty"`
	IntCode    int    `json:"intCode,omitempty"`
	Info       string `json:"info,omitempty"`
	Body       string `json:"-"`
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}

	parts := []string{}
	if e.Message != "" {
		parts = append(parts, e.Message)
	}
	if e.StatusCode != 0 {
		parts = append(parts, fmt.Sprintf("status=%d", e.StatusCode))
	}
	if e.Code != "" {
		parts = append(parts, "code="+e.Code)
	}
	if e.IntCode != 0 {
		parts = append(parts, fmt.Sprintf("intCode=%d", e.IntCode))
	}
	if e.Info != "" {
		parts = append(parts, "info="+e.Info)
	}
	if len(parts) == 0 {
		return "monpay api error"
	}
	return strings.Join(parts, ": ")
}

type (
	MonpayQrInput struct {
		Amount float64
	}
	MonpayQrRequest struct {
		Amount       float64 `json:"amount"`
		DisplayName  string  `json:"string"`
		GenerateUUID bool    `json:"generateUuid"`
		CallbackUrl  string  `json:"callbackUrl"`
	}
	MonpayQrResponse struct {
		Code   int            `json:"code"`
		Info   string         `json:"info"`
		Result MonpayResultQr `json:"result"`
	}
	MonpayResultQr struct {
		Qrcode string `json:"qrcode"`
		UUID   string `json:"uuid"`
	}

	MonpayCheckResponse struct {
		Code   int               `json:"code"`
		Info   string            `json:"info"`
		Result MonpayResultCheck `json:"result"`
	}
	MonpayResultCheck struct {
		UUID          string `json:"uuid"`
		UsedAt        int64  `json:"usedAt"`
		UsedByUd      int64  `json:"usedById"`
		TransactionId string `json:"transactionId"`
		Amount        int64  `json:"amount"`
		CreatedAt     int64  `json:"createdAt"`
		UserPhone     string `json:"userPhone"`
		UserAccountNo string `json:"userAccountNo"`
		UserVatId     string `json:"userVatId"`
		UsedAtUI      string `json:"usedAtUI"`
		CreatedAtUI   string `json:"createdAtUI"`
		AmountUI      string `json:"amountUI"`
	}

	MonpayCallback struct {
		Amount float64 `schema:"amount"`
		UUID   string  `schema:"uuid"`
		Status string  `schema:"status"`
		TnxId  string  `schema:"tnxId"`
	}
)

type (
	MiniAppAuthInput struct {
		RedirectURI string
		Code        string
		GrantType   string
	}

	MiniAppUserInfoResponse struct {
		Code    string                `json:"code"`
		IntCode int                   `json:"intCode"`
		Info    string                `json:"info"`
		Result  MiniAppUserInfoResult `json:"result"`
	}

	MiniAppUserInfoResult struct {
		UserID        int            `json:"userId"`
		UserPhone     FlexibleString `json:"userPhone"`
		UserEmail     string         `json:"userEmail"`
		UserFirstname string         `json:"userFirstname"`
		UserLastname  string         `json:"userLastname"`
	}

	MiniAppCreateInvoiceInput struct {
		RedirectUri      string
		Amount           float64
		ClientServiceUrl string
		Receiver         string
		InvoiceType      InvoiceType
		Description      string
		AccessToken      string
	}

	MiniAppCreateInvoiceRequest struct {
		RedirectUri      string      `json:"redirectUri"`                // Webhook буюу гүйлгээний үр дүн илгээгдэх буцах хаяг
		Amount           float64     `json:"amount"`                     // Дүн
		ClientServiceUrl string      `json:"clientServiceUrl,omitempty"` // Амжилттай гүйлгээний дараа backend-ээс дуудах webhook url.
		Receiver         string      `json:"receiver"`                   // Нэхэмжлэхийн төрлөөс хамаарч утга нь өөр өөр байна.
		InvoiceType      InvoiceType `json:"invoiceType"`                // P2B, P2P, B2B
		Description      string      `json:"description"`                // Тайлбар
	}

	MiniAppInvoiceResponse struct {
		Code    string               `json:"code"`    // Төлөвийн тайлбар код
		IntCode int                  `json:"intCode"` // Төлөвийн код
		Info    string               `json:"info"`    // Төлөвийн мэдээлэл
		Result  MiniAppInvoiceResult `json:"result"`
	}

	MiniAppInvoiceResult struct {
		ID              int         `json:"id"`          // Нэхэмжлэхийн давтагдашгүй id
		Receiver        string      `json:"receiver"`    // P2B салбар, P2P данс, B2B байгууллага
		Amount          float64     `json:"amount"`      // Дүн
		UserID          int         `json:"userId"`      // Төлөгч хэрэглэгчийн id
		MiniAppID       int         `json:"miniAppId"`   // Мини апп id
		CreateDate      time.Time   `json:"createDate"`  // Нэхэмжлэх үүссэн огноо
		UpdateDate      time.Time   `json:"updateDate"`  // Нэхэмжлэх засагдсан огноо
		Status          string      `json:"status"`      // NEW, PAID, FAILED
		Description     string      `json:"description"` // Нэхэмжлэхийн тайлбар
		TxnID           string      `json:"txnId"`       // Гүйлгээний дугаар
		StatusInfo      string      `json:"statusInfo"`  // Хэвлэж болохуйц мэдээлэл
		RedirectUri     string      `json:"redirectUri"` // Буцах url хаяг
		InvoiceType     InvoiceType `json:"invoiceType"`
		BankName        Bank        `json:"bankName"`        // Банк нэр (Only B2B connections)
		BankAccount     string      `json:"bankAccount"`     // Банкны дансны дугаар (Only B2B connections)
		BankAccountName string      `json:"bankAccountName"` // Данс эзэмшигчийн нэр (Only B2B connections)
	}

	DeeplinkCreateRequest  = MiniAppCreateInvoiceRequest
	DeeplinkCreateResponse = MiniAppInvoiceResponse
	DeeplinkCreateResult   = MiniAppInvoiceResult
	DeeplinkCheckResponse  = MiniAppInvoiceResponse
	DeeplinkCheckResult    = MiniAppInvoiceResult

	DeeplinkCallback struct {
		Amount    float64 `schema:"amount"`    // Нэхэмжилсэн дүн
		InvoiceId string  `schema:"invoiceId"` // Төлсөн нэхэмжлэхийн id
		Status    string  `schema:"status"`    // PAID, FAILED
		TnxId     string  `schema:"tnxId"`     // Гүйлгээ амжилттай болсон бол гүйлгээний дугаар
		Info      string  `schema:"info"`      // Хүнд уншигдахуйц гүйлгээний үр дүн
	}
)
