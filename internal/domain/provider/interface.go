package provider

import (
	"context"
	"time"

	"github.com/reveegate/reveegate/internal/domain/payment"
)

// PaymentRequest holds the request to create a payment
type PaymentRequest struct {
	OrderID       string
	Amount        int64
	PaymentMethod payment.Method
	CustomerName  string
	CustomerEmail string
	Description   string
	CallbackURL   string
	ExpiryTime    time.Time
}

// PaymentResponse holds the response from creating a payment
type PaymentResponse struct {
	ExternalID    string
	TransactionID string
	PaymentMethod payment.Method
	QRCodeURL     string
	VANumber      string
	DeepLink      string
	ExpiresAt     time.Time
	RawResponse   map[string]interface{}
}

// PaymentStatus holds the payment status from provider
type PaymentStatus struct {
	OrderID         string
	ExternalID      string
	TransactionID   string
	RawStatus       string
	Status          payment.Status
	TransactionTime time.Time
	Amount          int64
	PaymentMethod   payment.Method
}

// WebhookData holds parsed webhook data
type WebhookData struct {
	OrderID         string
	TransactionID   string
	TransactionTime time.Time
	Status          payment.Status
	Amount          int64
	PaymentMethod   payment.Method
	RawPayload      map[string]interface{}
}

// Provider defines the payment provider interface
type Provider interface {
	// GetName returns the provider name
	GetName() payment.Provider

	// CreatePayment creates a new payment
	CreatePayment(ctx context.Context, req PaymentRequest) (*PaymentResponse, error)

	// VerifyWebhook verifies the webhook signature
	VerifyWebhook(payload []byte, signature string) error

	// ParseWebhook parses the webhook payload
	ParseWebhook(payload []byte) (*WebhookData, error)

	// GetPaymentStatus gets the payment status from provider
	GetPaymentStatus(ctx context.Context, externalID string) (*PaymentStatus, error)

	// GetSupportedMethods returns the supported payment methods
	GetSupportedMethods() []payment.Method

	// IsMethodSupported checks if a payment method is supported
	IsMethodSupported(method payment.Method) bool
}

// ProviderFactory creates payment providers
type ProviderFactory interface {
	// GetProvider returns the appropriate provider for the given name
	GetProvider(name payment.Provider) (Provider, error)

	// GetProviderForMethod returns the appropriate provider for the given payment method
	GetProviderForMethod(method payment.Method) (Provider, error)

	// GetAllProviders returns all available providers
	GetAllProviders() []Provider
}
