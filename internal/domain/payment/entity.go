package payment

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Status represents the status of a payment
type Status string

const (
	StatusPending  Status = "pending"
	StatusPaid     Status = "paid"
	StatusExpired  Status = "expired"
	StatusFailed   Status = "failed"
	StatusRefunded Status = "refunded"
)

// Method represents the payment method
type Method string

const (
	MethodQRIS      Method = "qris"
	MethodGoPay     Method = "gopay"
	MethodDANA      Method = "dana"
	MethodOVO       Method = "ovo"
	MethodShopeePay Method = "shopeepay"
	MethodLinkAja   Method = "linkaja"
	MethodVABCA     Method = "va_bca"
	MethodVABNI     Method = "va_bni"
	MethodVAMandiri Method = "va_mandiri"
	MethodVABRI     Method = "va_bri"
	MethodVAPermata Method = "va_permata"
)

// Provider represents the payment provider
type Provider string

const (
	ProviderMidtrans Provider = "midtrans"
	ProviderXendit   Provider = "xendit"
)

// Payment represents a payment entity
type Payment struct {
	ID            uuid.UUID              `json:"id"`
	DonationID    uuid.UUID              `json:"donation_id"`
	Provider      Provider               `json:"provider"`
	ExternalID    string                 `json:"external_id"`
	PaymentMethod Method                 `json:"payment_method"`
	Method        Method                 `json:"method"` // Alias for PaymentMethod
	Amount        int64                  `json:"amount"`
	Status        Status                 `json:"status"`
	QRCodeURL     string                 `json:"qr_code_url,omitempty"`
	VANumber      string                 `json:"va_number,omitempty"`
	DeepLink      string                 `json:"deep_link,omitempty"`
	ExpiresAt     time.Time              `json:"expires_at"`
	PaidAt        *time.Time             `json:"paid_at,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
}

// NewPayment creates a new payment
func NewPayment(donationID uuid.UUID, provider Provider, method Method, amount int64, expiresAt time.Time) *Payment {
	now := time.Now()
	return &Payment{
		ID:            uuid.New(),
		DonationID:    donationID,
		Provider:      provider,
		ExternalID:    "", // Set by provider
		PaymentMethod: method,
		Method:        method, // Set both fields
		Amount:        amount,
		Status:        StatusPending,
		ExpiresAt:     expiresAt,
		Metadata:      make(map[string]interface{}),
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

// SetExternalID sets the external ID from the payment provider
func (p *Payment) SetExternalID(externalID string) {
	p.ExternalID = externalID
	p.UpdatedAt = time.Now()
}

// SetPaymentDetails sets the payment details based on method
func (p *Payment) SetPaymentDetails(qrCodeURL, vaNumber, deepLink string) {
	p.QRCodeURL = qrCodeURL
	p.VANumber = vaNumber
	p.DeepLink = deepLink
	p.UpdatedAt = time.Now()
}

// MarkAsPaid marks the payment as paid
func (p *Payment) MarkAsPaid() {
	now := time.Now()
	p.Status = StatusPaid
	p.PaidAt = &now
	p.UpdatedAt = now
}

// MarkAsExpired marks the payment as expired
func (p *Payment) MarkAsExpired() {
	p.Status = StatusExpired
	p.UpdatedAt = time.Now()
}

// MarkAsFailed marks the payment as failed
func (p *Payment) MarkAsFailed() {
	p.Status = StatusFailed
	p.UpdatedAt = time.Now()
}

// IsExpired checks if the payment has expired
func (p *Payment) IsExpired() bool {
	return time.Now().After(p.ExpiresAt) && p.Status == StatusPending
}

// IsPending checks if the payment is pending
func (p *Payment) IsPending() bool {
	return p.Status == StatusPending
}

// IsPaid checks if the payment is paid
func (p *Payment) IsPaid() bool {
	return p.Status == StatusPaid
}

// GenerateOrderID generates a unique order ID for the payment provider
func (p *Payment) GenerateOrderID() string {
	return "DONATION-" + p.ID.String()[:8]
}

// Repository defines the payment repository interface
type Repository interface {
	Create(ctx context.Context, payment *Payment) error
	GetByID(ctx context.Context, id uuid.UUID) (*Payment, error)
	GetByDonationID(ctx context.Context, donationID uuid.UUID) (*Payment, error)
	GetByExternalID(ctx context.Context, provider Provider, externalID string) (*Payment, error)
	Update(ctx context.Context, payment *Payment) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status Status) error
	GetPendingExpired(ctx context.Context) ([]*Payment, error)
	List(ctx context.Context, params ListPaymentsParams) (*ListPaymentsResult, error)
}

// ListPaymentsParams holds parameters for listing payments
type ListPaymentsParams struct {
	Status    *Status
	Provider  *Provider
	StartDate *time.Time
	EndDate   *time.Time
	Page      int
	Limit     int
}

// ListPaymentsResult holds the result of listing payments
type ListPaymentsResult struct {
	Payments   []*Payment `json:"payments"`
	Total      int64      `json:"total"`
	Page       int        `json:"page"`
	Limit      int        `json:"limit"`
	TotalPages int        `json:"total_pages"`
}

// WebhookLog represents a webhook log entry
type WebhookLog struct {
	ID           uuid.UUID              `json:"id"`
	Provider     Provider               `json:"provider"`
	EventType    string                 `json:"event_type"`
	ExternalID   string                 `json:"external_id"`
	StatusCode   int                    `json:"status_code"`
	RawPayload   map[string]interface{} `json:"raw_payload"`
	Payload      []byte                 `json:"payload,omitempty"`
	Headers      map[string]string      `json:"headers,omitempty"`
	Signature    string                 `json:"signature"`
	IPAddress    string                 `json:"ip_address"`
	Processed    bool                   `json:"processed"`
	ErrorMessage string                 `json:"error_message,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
}

// NewWebhookLog creates a new webhook log
func NewWebhookLog(provider Provider, eventType, externalID string, payload map[string]interface{}, signature, ip string) *WebhookLog {
	return &WebhookLog{
		ID:         uuid.New(),
		Provider:   provider,
		EventType:  eventType,
		ExternalID: externalID,
		RawPayload: payload,
		Signature:  signature,
		IPAddress:  ip,
		Processed:  false,
		CreatedAt:  time.Now(),
	}
}

// WebhookLogRepository defines the webhook log repository interface
type WebhookLogRepository interface {
	Create(ctx context.Context, log *WebhookLog) error
	GetByID(ctx context.Context, id uuid.UUID) (*WebhookLog, error)
	List(ctx context.Context, params ListWebhookLogsParams) (*ListWebhookLogsResult, error)
	MarkAsProcessed(ctx context.Context, id uuid.UUID, statusCode int, errorMsg string) error
}

// ListWebhookLogsParams holds parameters for listing webhook logs
type ListWebhookLogsParams struct {
	Provider  *Provider
	Processed *bool
	StartDate *time.Time
	EndDate   *time.Time
	Page      int
	Limit     int
}

// ListWebhookLogsResult holds the result of listing webhook logs
type ListWebhookLogsResult struct {
	Logs       []*WebhookLog `json:"logs"`
	Total      int64         `json:"total"`
	Page       int           `json:"page"`
	Limit      int           `json:"limit"`
	TotalPages int           `json:"total_pages"`
}
