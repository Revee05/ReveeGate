package dto

import (
	"time"

	"github.com/google/uuid"
)

// CreateDonationRequest represents the request to create a donation
type CreateDonationRequest struct {
	DonorName     string `json:"donor_name" validate:"max=100"`
	DonorEmail    string `json:"donor_email,omitempty" validate:"omitempty,email,max=255"`
	Message       string `json:"message,omitempty" validate:"max=500"`
	Amount        int64  `json:"amount" validate:"required,min=5000,max=100000000"`
	PaymentMethod string `json:"payment_method" validate:"required,oneof=qris gopay dana ovo shopeepay linkaja va_bca va_bni va_mandiri va_bri va_permata"`
}

// DonationResponse represents the response after creating a donation
type DonationResponse struct {
	ID          uuid.UUID    `json:"id"`
	DonationID  uuid.UUID    `json:"donation_id,omitempty"`
	DonorName   string       `json:"donor_name"`
	DonorEmail  string       `json:"donor_email,omitempty"`
	Message     string       `json:"message,omitempty"`
	Amount      int64        `json:"amount"`
	Status      string       `json:"status"`
	CreatedAt   time.Time    `json:"created_at"`
	PaidAt      *time.Time   `json:"paid_at,omitempty"`
	PaymentInfo *PaymentInfo `json:"payment_info,omitempty"`
}

// PaymentInfo represents payment details in donation response
type PaymentInfo struct {
	PaymentID      uuid.UUID `json:"payment_id"`
	Provider       string    `json:"provider"`
	Method         string    `json:"method"`
	Status         string    `json:"status"`
	QRCodeURL      string    `json:"qr_code_url,omitempty"`
	VANumber       string    `json:"va_number,omitempty"`
	DeepLink       string    `json:"deep_link,omitempty"`
	PaymentPageURL string    `json:"payment_page_url,omitempty"`
	ExpiresAt      time.Time `json:"expires_at"`
}

// DonationStatusResponse represents a donation status response
type DonationStatusResponse struct {
	DonationID uuid.UUID  `json:"donation_id"`
	DonorName  string     `json:"donor_name"`
	Message    string     `json:"message,omitempty"`
	Amount     int64      `json:"amount"`
	Status     string     `json:"status"`
	PaidAt     *time.Time `json:"paid_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// ListDonationsRequest represents the request to list donations
type ListDonationsRequest struct {
	Status    string     `query:"status" validate:"omitempty,oneof=pending completed expired failed cancelled"`
	StartDate *time.Time `query:"start_date"`
	EndDate   *time.Time `query:"end_date"`
	Page      int        `query:"page" validate:"omitempty,min=1"`
	Limit     int        `query:"limit" validate:"omitempty,min=1,max=100"`
}

// ListDonationsResponse represents the response for listing donations
type ListDonationsResponse struct {
	Donations  []DonationStatusResponse `json:"donations"`
	Pagination PaginationResponse       `json:"pagination"`
}

// PaginationResponse represents pagination info
type PaginationResponse struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

// DonationStatsResponse represents donation statistics
type DonationStatsResponse struct {
	TotalDonations int64   `json:"total_donations"`
	TotalAmount    int64   `json:"total_amount"`
	AverageAmount  float64 `json:"average_amount"`
	CompletedCount int64   `json:"completed_count"`
	PendingCount   int64   `json:"pending_count"`
	FailedCount    int64   `json:"failed_count"`
}

// MidtransWebhook represents the webhook payload from Midtrans
type MidtransWebhook struct {
	OrderID           string `json:"order_id"`
	TransactionID     string `json:"transaction_id"`
	TransactionStatus string `json:"transaction_status"`
	TransactionTime   string `json:"transaction_time"`
	SettlementTime    string `json:"settlement_time,omitempty"`
	StatusCode        string `json:"status_code"`
	GrossAmount       string `json:"gross_amount"`
	PaymentType       string `json:"payment_type"`
	SignatureKey      string `json:"signature_key"`
	FraudStatus       string `json:"fraud_status,omitempty"`
}

// XenditWebhook represents the webhook payload from Xendit
type XenditWebhook struct {
	ID             string `json:"id"`
	ExternalID     string `json:"external_id"`
	PayerEmail     string `json:"payer_email,omitempty"`
	Description    string `json:"description,omitempty"`
	Amount         int64  `json:"amount"`
	Status         string `json:"status"`
	PaidAt         string `json:"paid_at,omitempty"`
	PaymentMethod  string `json:"payment_method"`
	PaymentChannel string `json:"payment_channel,omitempty"`
}

// ReconcilePaymentRequest represents the request to manually reconcile a payment
type ReconcilePaymentRequest struct {
	Action string `json:"action" validate:"required,oneof=mark_as_paid mark_as_failed"`
	Reason string `json:"reason" validate:"required,min=10,max=500"`
}

// ReconcilePaymentResponse represents the response after reconciliation
type ReconcilePaymentResponse struct {
	Status        string `json:"status"`
	PaymentStatus string `json:"payment_status"`
}

// ListWebhookLogsRequest represents the request to list webhook logs
type ListWebhookLogsRequest struct {
	Provider  string     `query:"provider" validate:"omitempty,oneof=midtrans xendit"`
	Processed *bool      `query:"processed"`
	StartDate *time.Time `query:"start_date"`
	EndDate   *time.Time `query:"end_date"`
	Page      int        `query:"page" validate:"omitempty,min=1"`
	Limit     int        `query:"limit" validate:"omitempty,min=1,max=100"`
}

// WebhookLogResponse represents a webhook log entry
type WebhookLogResponse struct {
	ID           uuid.UUID              `json:"id"`
	Provider     string                 `json:"provider"`
	EventType    string                 `json:"event_type"`
	ExternalID   string                 `json:"external_id"`
	StatusCode   int                    `json:"status_code"`
	RawPayload   map[string]interface{} `json:"raw_payload"`
	Processed    bool                   `json:"processed"`
	ErrorMessage string                 `json:"error_message,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
}

// ListWebhookLogsResponse represents the response for listing webhook logs
type ListWebhookLogsResponse struct {
	Logs       []WebhookLogResponse `json:"logs"`
	Pagination PaginationResponse   `json:"pagination"`
}

// AdminLoginRequest represents admin login request
type AdminLoginRequest struct {
	Email    string `json:"email" validate:"omitempty,email,max=100"`
	Username string `json:"username" validate:"omitempty,min=3,max=100"`
	Password string `json:"password" validate:"required,min=8,max=100"`
}

// AdminLoginResponse represents admin login response
type AdminLoginResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresIn    int       `json:"expires_in"`
	TokenType    string    `json:"token_type"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
}

// RefreshTokenRequest represents a refresh token request
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
	Code    string `json:"code,omitempty"`
}

// SuccessResponse represents a generic success response
type SuccessResponse struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// HealthResponse represents a health check response
type HealthResponse struct {
	Status    string            `json:"status"`
	Version   string            `json:"version,omitempty"`
	Timestamp string            `json:"timestamp"`
	Services  map[string]string `json:"services,omitempty"`
}

// OverlayTokenRequest represents a request to create an overlay token
type OverlayTokenRequest struct {
	Description string `json:"description" validate:"max=255"`
}

// OverlayTokenResponse represents an overlay token response
type OverlayTokenResponse struct {
	ID          uuid.UUID  `json:"id"`
	Token       string     `json:"token"`
	Description string     `json:"description,omitempty"`
	IsActive    bool       `json:"is_active"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}
