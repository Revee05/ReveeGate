package xendit

import (
	"bytes"
	"context"
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
	baseURL = "https://api.xendit.co"
)

// Provider implements Xendit payment provider
type Provider struct {
	secretKey  string
	publicKey  string
	httpClient *http.Client
}

// NewProvider creates a new Xendit provider
func NewProvider(cfg config.XenditConfig) *Provider {
	return &Provider{
		secretKey: cfg.SecretKey,
		publicKey: cfg.PublicKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetName returns the provider name
func (p *Provider) GetName() payment.Provider {
	return payment.ProviderXendit
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
		payment.MethodOVO,
		payment.MethodDANA,
		payment.MethodLinkAja,
	}
}

// CreatePayment creates a new payment
func (p *Provider) CreatePayment(ctx context.Context, req provider.PaymentRequest) (*provider.PaymentResponse, error) {
	switch {
	case isQRIS(req.PaymentMethod):
		return p.createQRISPayment(ctx, req)
	case isVA(req.PaymentMethod):
		return p.createVAPayment(ctx, req)
	case isEWallet(req.PaymentMethod):
		return p.createEWalletPayment(ctx, req)
	default:
		return nil, fmt.Errorf("unsupported payment method: %s", req.PaymentMethod)
	}
}

// GetPaymentStatus gets the status of a payment
func (p *Provider) GetPaymentStatus(ctx context.Context, orderID string) (*provider.PaymentStatus, error) {
	// Xendit uses different endpoints for different payment types
	// For simplicity, we'll check invoice status
	endpoint := fmt.Sprintf("/v2/invoices?external_id=%s", orderID)

	resp, err := p.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get payment status: %w", err)
	}

	// Parse array response
	invoices, ok := resp.([]interface{})
	if !ok || len(invoices) == 0 {
		return nil, fmt.Errorf("invoice not found")
	}

	invoice := invoices[0].(map[string]interface{})

	status := &provider.PaymentStatus{
		OrderID:       invoice["external_id"].(string),
		TransactionID: invoice["id"].(string),
		RawStatus:     invoice["status"].(string),
	}

	// Map status
	switch invoice["status"].(string) {
	case "PAID", "SETTLED":
		status.Status = payment.StatusPaid
	case "PENDING":
		status.Status = payment.StatusPending
	case "EXPIRED":
		status.Status = payment.StatusExpired
	default:
		status.Status = payment.StatusPending
	}

	return status, nil
}

// createQRISPayment creates a QRIS payment
func (p *Provider) createQRISPayment(ctx context.Context, req provider.PaymentRequest) (*provider.PaymentResponse, error) {
	body := map[string]interface{}{
		"external_id":  req.OrderID,
		"type":         "DYNAMIC",
		"channel_code": "QRIS",
		"amount":       req.Amount,
		"currency":     "IDR",
		"expires_at":   req.ExpiryTime.Format(time.RFC3339),
		"metadata": map[string]interface{}{
			"customer_name":  req.CustomerName,
			"customer_email": req.CustomerEmail,
		},
	}

	respData, err := p.doRequest(ctx, "POST", "/qr_codes", body)
	if err != nil {
		return nil, err
	}

	resp := respData.(map[string]interface{})

	expiresAt, _ := time.Parse(time.RFC3339, resp["expires_at"].(string))

	return &provider.PaymentResponse{
		ExternalID:    resp["external_id"].(string),
		TransactionID: resp["id"].(string),
		QRCodeURL:     resp["qr_string"].(string),
		ExpiresAt:     expiresAt,
	}, nil
}

// createVAPayment creates a Virtual Account payment
func (p *Provider) createVAPayment(ctx context.Context, req provider.PaymentRequest) (*provider.PaymentResponse, error) {
	bankCode := mapMethodToBank(req.PaymentMethod)

	body := map[string]interface{}{
		"external_id":     req.OrderID,
		"bank_code":       bankCode,
		"name":            req.CustomerName,
		"expected_amount": req.Amount,
		"is_closed":       true,
		"is_single_use":   true,
		"expiration_date": req.ExpiryTime.Format(time.RFC3339),
	}

	respData, err := p.doRequest(ctx, "POST", "/callback_virtual_accounts", body)
	if err != nil {
		return nil, err
	}

	resp := respData.(map[string]interface{})

	expiresAt, _ := time.Parse(time.RFC3339, resp["expiration_date"].(string))

	return &provider.PaymentResponse{
		ExternalID:    resp["external_id"].(string),
		TransactionID: resp["id"].(string),
		VANumber:      resp["account_number"].(string),
		ExpiresAt:     expiresAt,
	}, nil
}

// createEWalletPayment creates an E-Wallet payment
func (p *Provider) createEWalletPayment(ctx context.Context, req provider.PaymentRequest) (*provider.PaymentResponse, error) {
	channelCode := mapMethodToChannel(req.PaymentMethod)

	body := map[string]interface{}{
		"reference_id":    req.OrderID,
		"currency":        "IDR",
		"amount":          req.Amount,
		"checkout_method": "ONE_TIME_PAYMENT",
		"channel_code":    channelCode,
		"channel_properties": map[string]interface{}{
			"success_redirect_url": "https://reveegate.com/donation/success",
			"failure_redirect_url": "https://reveegate.com/donation/failed",
		},
		"metadata": map[string]interface{}{
			"customer_name":  req.CustomerName,
			"customer_email": req.CustomerEmail,
		},
	}

	respData, err := p.doRequest(ctx, "POST", "/ewallets/charges", body)
	if err != nil {
		return nil, err
	}

	resp := respData.(map[string]interface{})

	var deepLink string
	if actions, ok := resp["actions"].(map[string]interface{}); ok {
		if mobileDeepLink, ok := actions["mobile_deeplink_checkout_url"].(string); ok {
			deepLink = mobileDeepLink
		} else if desktopLink, ok := actions["desktop_web_checkout_url"].(string); ok {
			deepLink = desktopLink
		}
	}

	return &provider.PaymentResponse{
		ExternalID:    resp["reference_id"].(string),
		TransactionID: resp["id"].(string),
		DeepLink:      deepLink,
		ExpiresAt:     req.ExpiryTime,
	}, nil
}

// doRequest makes an HTTP request to Xendit API
func (p *Provider) doRequest(ctx context.Context, method, endpoint string, body interface{}) (interface{}, error) {
	url := baseURL + endpoint

	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(p.secretKey, "")

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

	// Check for error response
	if resp.StatusCode >= 400 {
		var errResp map[string]interface{}
		json.Unmarshal(respBody, &errResp)
		errMsg := "Unknown error"
		if msg, ok := errResp["message"].(string); ok {
			errMsg = msg
		}
		return nil, fmt.Errorf("xendit error (%d): %s", resp.StatusCode, errMsg)
	}

	// Parse response - could be array or object
	var result interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}

// Helper functions

func isQRIS(method payment.Method) bool {
	return method == payment.MethodQRIS
}

func isVA(method payment.Method) bool {
	switch method {
	case payment.MethodVABCA, payment.MethodVABNI, payment.MethodVABRI,
		payment.MethodVAMandiri, payment.MethodVAPermata:
		return true
	}
	return false
}

func isEWallet(method payment.Method) bool {
	switch method {
	case payment.MethodGoPay, payment.MethodOVO, payment.MethodDANA,
		payment.MethodShopeePay, payment.MethodLinkAja:
		return true
	}
	return false
}

func mapMethodToBank(method payment.Method) string {
	switch method {
	case payment.MethodVABCA:
		return "BCA"
	case payment.MethodVABNI:
		return "BNI"
	case payment.MethodVABRI:
		return "BRI"
	case payment.MethodVAMandiri:
		return "MANDIRI"
	case payment.MethodVAPermata:
		return "PERMATA"
	default:
		return ""
	}
}

func mapMethodToChannel(method payment.Method) string {
	switch method {
	case payment.MethodOVO:
		return "ID_OVO"
	case payment.MethodDANA:
		return "ID_DANA"
	case payment.MethodShopeePay:
		return "ID_SHOPEEPAY"
	case payment.MethodLinkAja:
		return "ID_LINKAJA"
	case payment.MethodGoPay:
		return "ID_GOPAY"
	default:
		return ""
	}
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
	// Xendit uses callback token for verification
	// This should be checked against X-CALLBACK-TOKEN header
	return nil
}

// ParseWebhook parses the webhook payload
func (p *Provider) ParseWebhook(payload []byte) (*provider.WebhookData, error) {
	var data map[string]interface{}
	if err := json.Unmarshal(payload, &data); err != nil {
		return nil, fmt.Errorf("failed to parse webhook payload: %w", err)
	}

	externalID, _ := data["external_id"].(string)
	id, _ := data["id"].(string)
	status, _ := data["status"].(string)
	paidAt, _ := data["paid_at"].(string)

	// Parse amount
	var amount int64
	if a, ok := data["amount"].(float64); ok {
		amount = int64(a)
	}

	// Parse paid_at time
	parsedTime, _ := time.Parse(time.RFC3339, paidAt)

	// Map status
	var paymentStatus payment.Status
	switch status {
	case "PAID", "SETTLED":
		paymentStatus = payment.StatusPaid
	case "PENDING", "ACTIVE":
		paymentStatus = payment.StatusPending
	case "EXPIRED":
		paymentStatus = payment.StatusExpired
	case "FAILED":
		paymentStatus = payment.StatusFailed
	default:
		paymentStatus = payment.StatusPending
	}

	return &provider.WebhookData{
		OrderID:         externalID,
		TransactionID:   id,
		TransactionTime: parsedTime,
		Status:          paymentStatus,
		Amount:          amount,
		RawPayload:      data,
	}, nil
}
