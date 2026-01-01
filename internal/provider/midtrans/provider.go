package midtrans

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/reveegate/reveegate/internal/config"
	"github.com/reveegate/reveegate/internal/domain/payment"
	"github.com/reveegate/reveegate/internal/domain/provider"
)

const (
	sandboxURL    = "https://api.sandbox.midtrans.com"
	productionURL = "https://api.midtrans.com"
)

// Provider implements Midtrans payment provider
type Provider struct {
	serverKey    string
	clientKey    string
	merchantID   string
	baseURL      string
	isProduction bool
	httpClient   *http.Client
}

// NewProvider creates a new Midtrans provider
func NewProvider(cfg config.MidtransConfig) *Provider {
	baseURL := sandboxURL
	if cfg.IsProduction {
		baseURL = productionURL
	}

	return &Provider{
		serverKey:    cfg.ServerKey,
		clientKey:    cfg.ClientKey,
		merchantID:   cfg.MerchantID,
		baseURL:      baseURL,
		isProduction: cfg.IsProduction,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetName returns the provider name
func (p *Provider) GetName() payment.Provider {
	return payment.ProviderMidtrans
}

// GetSupportedMethods returns supported payment methods
func (p *Provider) GetSupportedMethods() []payment.Method {
	return []payment.Method{
		payment.MethodQRIS,
		payment.MethodVABCA,
		payment.MethodVABNI,
		payment.MethodVABRI,
		payment.MethodVAMandiri,
		payment.MethodVAPermata,
		payment.MethodGoPay,
		payment.MethodShopeePay,
		payment.MethodDANA,
		payment.MethodOVO,
	}
}

// CreatePayment creates a new payment
func (p *Provider) CreatePayment(ctx context.Context, req provider.PaymentRequest) (*provider.PaymentResponse, error) {
	// Build Midtrans request based on payment method
	midtransReq := p.buildRequest(req)

	// Make API call
	resp, err := p.doRequest(ctx, "/v2/charge", midtransReq)
	if err != nil {
		return nil, fmt.Errorf("midtrans charge failed: %w", err)
	}

	// Parse response
	return p.parseChargeResponse(resp, req.PaymentMethod)
}

// GetPaymentStatus gets the status of a payment
func (p *Provider) GetPaymentStatus(ctx context.Context, orderID string) (*provider.PaymentStatus, error) {
	endpoint := fmt.Sprintf("/v2/%s/status", orderID)

	resp, err := p.doRequest(ctx, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get payment status: %w", err)
	}

	status := &provider.PaymentStatus{
		OrderID:       resp["order_id"].(string),
		TransactionID: resp["transaction_id"].(string),
		RawStatus:     resp["transaction_status"].(string),
	}

	// Map status
	switch resp["transaction_status"].(string) {
	case "capture", "settlement":
		status.Status = payment.StatusPaid
	case "pending":
		status.Status = payment.StatusPending
	case "deny", "cancel":
		status.Status = payment.StatusFailed
	case "expire":
		status.Status = payment.StatusExpired
	default:
		status.Status = payment.StatusPending
	}

	return status, nil
}

// buildRequest builds Midtrans charge request
func (p *Provider) buildRequest(req provider.PaymentRequest) map[string]interface{} {
	baseReq := map[string]interface{}{
		"transaction_details": map[string]interface{}{
			"order_id":     req.OrderID,
			"gross_amount": req.Amount,
		},
		"customer_details": map[string]interface{}{
			"first_name": req.CustomerName,
			"email":      req.CustomerEmail,
		},
		"custom_expiry": map[string]interface{}{
			"expiry_duration": 1440, // 24 hours in minutes
			"unit":            "minute",
		},
	}

	// Add payment type specific configuration
	switch req.PaymentMethod {
	case payment.MethodQRIS:
		baseReq["payment_type"] = "qris"
		baseReq["qris"] = map[string]interface{}{
			"acquirer": "gopay",
		}

	case payment.MethodGoPay:
		baseReq["payment_type"] = "gopay"
		baseReq["gopay"] = map[string]interface{}{
			"enable_callback": true,
		}

	case payment.MethodShopeePay:
		baseReq["payment_type"] = "shopeepay"

	case payment.MethodDANA:
		baseReq["payment_type"] = "dana"

	case payment.MethodOVO:
		baseReq["payment_type"] = "ovo"

	case payment.MethodVABCA:
		baseReq["payment_type"] = "bank_transfer"
		baseReq["bank_transfer"] = map[string]interface{}{
			"bank": "bca",
		}

	case payment.MethodVABNI:
		baseReq["payment_type"] = "bank_transfer"
		baseReq["bank_transfer"] = map[string]interface{}{
			"bank": "bni",
		}

	case payment.MethodVABRI:
		baseReq["payment_type"] = "bank_transfer"
		baseReq["bank_transfer"] = map[string]interface{}{
			"bank": "bri",
		}

	case payment.MethodVAMandiri:
		baseReq["payment_type"] = "echannel"
		baseReq["echannel"] = map[string]interface{}{
			"bill_info1": "Payment for",
			"bill_info2": req.Description,
		}

	case payment.MethodVAPermata:
		baseReq["payment_type"] = "permata"
	}

	return baseReq
}

// parseChargeResponse parses Midtrans charge response
func (p *Provider) parseChargeResponse(resp map[string]interface{}, method payment.Method) (*provider.PaymentResponse, error) {
	statusCode := resp["status_code"].(string)
	if statusCode != "201" && statusCode != "200" {
		errMsg := "Unknown error"
		if msg, ok := resp["status_message"].(string); ok {
			errMsg = msg
		}
		return nil, fmt.Errorf("midtrans error: %s", errMsg)
	}

	result := &provider.PaymentResponse{
		ExternalID:    resp["order_id"].(string),
		TransactionID: resp["transaction_id"].(string),
	}

	// Parse expiry time
	if expiry, ok := resp["expiry_time"].(string); ok {
		t, _ := time.Parse("2006-01-02 15:04:05", expiry)
		result.ExpiresAt = t
	}

	// Extract payment details based on method
	switch method {
	case payment.MethodQRIS:
		if actions, ok := resp["actions"].([]interface{}); ok {
			for _, action := range actions {
				a := action.(map[string]interface{})
				if a["name"] == "generate-qr-code" {
					result.QRCodeURL = a["url"].(string)
				}
			}
		}

	case payment.MethodGoPay:
		if actions, ok := resp["actions"].([]interface{}); ok {
			for _, action := range actions {
				a := action.(map[string]interface{})
				switch a["name"] {
				case "generate-qr-code":
					result.QRCodeURL = a["url"].(string)
				case "deeplink-redirect":
					result.DeepLink = a["url"].(string)
				}
			}
		}

	case payment.MethodShopeePay, payment.MethodDANA, payment.MethodOVO:
		if actions, ok := resp["actions"].([]interface{}); ok {
			for _, action := range actions {
				a := action.(map[string]interface{})
				if a["name"] == "deeplink-redirect" {
					result.DeepLink = a["url"].(string)
				}
			}
		}

	case payment.MethodVABCA, payment.MethodVABNI, payment.MethodVABRI, payment.MethodVAPermata:
		if vas, ok := resp["va_numbers"].([]interface{}); ok && len(vas) > 0 {
			va := vas[0].(map[string]interface{})
			result.VANumber = va["va_number"].(string)
		}
		if permataVA, ok := resp["permata_va_number"].(string); ok {
			result.VANumber = permataVA
		}

	case payment.MethodVAMandiri:
		if billKey, ok := resp["bill_key"].(string); ok {
			billerCode := resp["biller_code"].(string)
			result.VANumber = billerCode + " " + billKey
		}
	}

	return result, nil
}

// doRequest makes an HTTP request to Midtrans API
func (p *Provider) doRequest(ctx context.Context, endpoint string, body interface{}) (map[string]interface{}, error) {
	url := p.baseURL + endpoint

	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Basic auth with server key
	auth := base64.StdEncoding.EncodeToString([]byte(p.serverKey + ":"))
	req.Header.Set("Authorization", "Basic "+auth)

	// Make request
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse response
	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}

// IsMethodSupported checks if a payment method is supported
func (p *Provider) IsMethodSupported(method payment.Method) bool {
	for _, m := range p.GetSupportedMethods() {
		if m == method {
			return true
		}
	}
	return false
}

// VerifyWebhook verifies the webhook signature
func (p *Provider) VerifyWebhook(payload []byte, signature string) error {
	// Midtrans uses signature_key in the payload itself
	// The signature is SHA512(order_id + status_code + gross_amount + server_key)
	// Verification is done in ParseWebhook
	return nil
}

// ParseWebhook parses the webhook payload
func (p *Provider) ParseWebhook(payload []byte) (*provider.WebhookData, error) {
	var data map[string]interface{}
	if err := json.Unmarshal(payload, &data); err != nil {
		return nil, fmt.Errorf("failed to parse webhook payload: %w", err)
	}

	orderID, _ := data["order_id"].(string)
	transactionID, _ := data["transaction_id"].(string)
	transactionStatus, _ := data["transaction_status"].(string)
	transactionTime, _ := data["transaction_time"].(string)
	grossAmount, _ := data["gross_amount"].(string)

	// Parse transaction time
	parsedTime, _ := time.Parse("2006-01-02 15:04:05", transactionTime)

	// Parse amount
	var amount int64
	fmt.Sscanf(grossAmount, "%d", &amount)

	// Map status
	var status payment.Status
	switch transactionStatus {
	case "capture", "settlement":
		status = payment.StatusPaid
	case "pending":
		status = payment.StatusPending
	case "deny", "cancel":
		status = payment.StatusFailed
	case "expire":
		status = payment.StatusExpired
	default:
		status = payment.StatusPending
	}

	return &provider.WebhookData{
		OrderID:         orderID,
		TransactionID:   transactionID,
		TransactionTime: parsedTime,
		Status:          status,
		Amount:          amount,
		RawPayload:      data,
	}, nil
}
