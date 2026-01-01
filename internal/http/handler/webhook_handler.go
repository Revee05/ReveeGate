package handler

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/reveegate/reveegate/internal/config"
	"github.com/reveegate/reveegate/internal/domain/payment"
	"github.com/reveegate/reveegate/internal/http/dto"
	"github.com/reveegate/reveegate/internal/service"
)

// WebhookHandler handles payment provider webhook callbacks
type WebhookHandler struct {
	donationService *service.DonationService
	webhookLogRepo  payment.WebhookLogRepository
	config          *config.Config
	logger          *slog.Logger
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(
	donationService *service.DonationService,
	webhookLogRepo payment.WebhookLogRepository,
	cfg *config.Config,
	logger *slog.Logger,
) *WebhookHandler {
	return &WebhookHandler{
		donationService: donationService,
		webhookLogRepo:  webhookLogRepo,
		config:          cfg,
		logger:          logger,
	}
}

// HandleMidtrans handles Midtrans webhook POST /api/v1/webhooks/midtrans
func (h *WebhookHandler) HandleMidtrans(w http.ResponseWriter, r *http.Request) {
	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error("failed to read webhook body", "error", err)
		h.respondError(w, http.StatusBadRequest, "INVALID_BODY", "Failed to read request body")
		return
	}

	// Log webhook
	h.logWebhook(r, payment.ProviderMidtrans, body)

	// Parse webhook
	var webhook dto.MidtransWebhook
	if err := json.Unmarshal(body, &webhook); err != nil {
		h.logger.Error("failed to parse webhook", "error", err)
		h.respondError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid webhook payload")
		return
	}

	// Verify signature
	if !h.verifyMidtransSignature(webhook) {
		h.logger.Warn("invalid midtrans signature",
			"order_id", webhook.OrderID,
			"signature", webhook.SignatureKey,
		)
		h.respondError(w, http.StatusUnauthorized, "INVALID_SIGNATURE", "Invalid webhook signature")
		return
	}

	// Map Midtrans status to internal status
	status := h.mapMidtransStatus(webhook.TransactionStatus, webhook.FraudStatus)

	// Parse paid time
	var paidAt time.Time
	if webhook.SettlementTime != "" {
		paidAt, _ = time.Parse("2006-01-02 15:04:05", webhook.SettlementTime)
	}

	// Process webhook
	err = h.donationService.ProcessWebhook(r.Context(), service.ProcessWebhookParams{
		Provider:      payment.ProviderMidtrans,
		OrderID:       webhook.OrderID,
		TransactionID: webhook.TransactionID,
		Status:        status,
		PaidAt:        paidAt,
		RawPayload:    body,
	})
	if err != nil {
		h.logger.Error("failed to process midtrans webhook",
			"order_id", webhook.OrderID,
			"error", err,
		)
		// Return 200 to prevent retries for non-retriable errors
		h.respondJSON(w, http.StatusOK, map[string]string{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	h.logger.Info("midtrans webhook processed",
		"order_id", webhook.OrderID,
		"transaction_id", webhook.TransactionID,
		"status", status,
	)

	h.respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// HandleXendit handles Xendit webhook POST /api/v1/webhooks/xendit
func (h *WebhookHandler) HandleXendit(w http.ResponseWriter, r *http.Request) {
	// Verify callback token
	callbackToken := r.Header.Get("x-callback-token")
	if callbackToken != h.config.Xendit.WebhookToken {
		h.logger.Warn("invalid xendit callback token")
		h.respondError(w, http.StatusUnauthorized, "INVALID_TOKEN", "Invalid callback token")
		return
	}

	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error("failed to read webhook body", "error", err)
		h.respondError(w, http.StatusBadRequest, "INVALID_BODY", "Failed to read request body")
		return
	}

	// Log webhook
	h.logWebhook(r, payment.ProviderXendit, body)

	// Parse webhook
	var webhook dto.XenditWebhook
	if err := json.Unmarshal(body, &webhook); err != nil {
		h.logger.Error("failed to parse webhook", "error", err)
		h.respondError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid webhook payload")
		return
	}

	// Map Xendit status to internal status
	status := h.mapXenditStatus(webhook.Status)

	// Parse paid time
	var paidAt time.Time
	if webhook.PaidAt != "" {
		paidAt, _ = time.Parse(time.RFC3339, webhook.PaidAt)
	}

	// Process webhook
	err = h.donationService.ProcessWebhook(r.Context(), service.ProcessWebhookParams{
		Provider:      payment.ProviderXendit,
		OrderID:       webhook.ExternalID,
		TransactionID: webhook.ID,
		Status:        status,
		PaidAt:        paidAt,
		RawPayload:    body,
	})
	if err != nil {
		h.logger.Error("failed to process xendit webhook",
			"external_id", webhook.ExternalID,
			"error", err,
		)
		h.respondJSON(w, http.StatusOK, map[string]string{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	h.logger.Info("xendit webhook processed",
		"external_id", webhook.ExternalID,
		"id", webhook.ID,
		"status", status,
	)

	h.respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// verifyMidtransSignature verifies Midtrans webhook signature
func (h *WebhookHandler) verifyMidtransSignature(webhook dto.MidtransWebhook) bool {
	// SHA512(order_id + status_code + gross_amount + ServerKey)
	data := webhook.OrderID + webhook.StatusCode + webhook.GrossAmount + h.config.Midtrans.ServerKey

	hash := sha512.New()
	hash.Write([]byte(data))
	expectedSignature := hex.EncodeToString(hash.Sum(nil))

	return hmac.Equal([]byte(expectedSignature), []byte(webhook.SignatureKey))
}

// mapMidtransStatus maps Midtrans status to internal status
func (h *WebhookHandler) mapMidtransStatus(transactionStatus, fraudStatus string) payment.Status {
	switch transactionStatus {
	case "capture":
		if fraudStatus == "accept" || fraudStatus == "" {
			return payment.StatusPaid
		}
		return payment.StatusPending
	case "settlement":
		return payment.StatusPaid
	case "pending":
		return payment.StatusPending
	case "deny", "cancel":
		return payment.StatusFailed
	case "expire":
		return payment.StatusExpired
	default:
		return payment.StatusPending
	}
}

// mapXenditStatus maps Xendit status to internal status
func (h *WebhookHandler) mapXenditStatus(status string) payment.Status {
	switch status {
	case "PAID", "SETTLED", "COMPLETED":
		return payment.StatusPaid
	case "PENDING", "ACTIVE":
		return payment.StatusPending
	case "EXPIRED":
		return payment.StatusExpired
	case "FAILED":
		return payment.StatusFailed
	default:
		return payment.StatusPending
	}
}

// logWebhook logs webhook to database
func (h *WebhookHandler) logWebhook(r *http.Request, provider payment.Provider, body []byte) {
	log := &payment.WebhookLog{
		Provider:  provider,
		EventType: r.Header.Get("x-event-type"),
		Payload:   body,
		Headers:   h.extractHeaders(r),
		IPAddress: h.getClientIP(r),
		CreatedAt: time.Now(),
	}

	if err := h.webhookLogRepo.Create(r.Context(), log); err != nil {
		h.logger.Error("failed to log webhook", "error", err)
	}
}

// extractHeaders extracts relevant headers
func (h *WebhookHandler) extractHeaders(r *http.Request) map[string]string {
	headers := make(map[string]string)

	// Extract relevant headers (not all for security)
	relevantHeaders := []string{
		"Content-Type",
		"User-Agent",
		"X-Request-Id",
		"X-Callback-Token",
		"X-Event-Type",
	}

	for _, name := range relevantHeaders {
		if val := r.Header.Get(name); val != "" {
			// Mask sensitive headers
			if name == "X-Callback-Token" {
				val = maskString(val)
			}
			headers[name] = val
		}
	}

	return headers
}

// getClientIP gets client IP from request
func (h *WebhookHandler) getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		// Return first IP in the chain
		return forwarded
	}

	// Check X-Real-IP header
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	return r.RemoteAddr
}

// maskString masks a string for logging
func maskString(s string) string {
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + "****" + s[len(s)-4:]
}

// respondJSON sends JSON response
func (h *WebhookHandler) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// respondError sends error response
func (h *WebhookHandler) respondError(w http.ResponseWriter, status int, code, message string) {
	h.respondJSON(w, status, dto.ErrorResponse{
		Error:   code,
		Message: message,
	})
}

// VerifyWebhook handles POST /api/v1/webhooks/verify for testing
func (h *WebhookHandler) VerifyWebhook(w http.ResponseWriter, r *http.Request) {
	// This endpoint is for webhook verification during setup
	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"message":   "Webhook endpoint is active",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// SimulatePaidWebhook handles POST /api/v1/webhooks/simulate (dev only)
func (h *WebhookHandler) SimulatePaidWebhook(w http.ResponseWriter, r *http.Request) {
	if h.config.App.Environment == "production" {
		h.respondError(w, http.StatusForbidden, "FORBIDDEN", "Not available in production")
		return
	}

	var req struct {
		DonationID string `json:"donation_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid request body")
		return
	}

	// Process as if paid
	err := h.donationService.ProcessWebhook(r.Context(), service.ProcessWebhookParams{
		Provider:      payment.ProviderMidtrans,
		OrderID:       fmt.Sprintf("DON-%s", req.DonationID),
		TransactionID: fmt.Sprintf("SIM-%d", time.Now().UnixNano()),
		Status:        payment.StatusPaid,
		PaidAt:        time.Now(),
		RawPayload:    []byte(`{"simulated": true}`),
	})
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "PROCESS_FAILED", err.Error())
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"message": "Simulated payment processed",
	})
}
