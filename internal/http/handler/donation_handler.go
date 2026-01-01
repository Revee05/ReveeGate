package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"github.com/reveegate/reveegate/internal/domain/donation"
	"github.com/reveegate/reveegate/internal/domain/payment"
	"github.com/reveegate/reveegate/internal/http/dto"
	"github.com/reveegate/reveegate/internal/service"
)

// DonationHandler handles donation HTTP requests
type DonationHandler struct {
	donationService *service.DonationService
	validator       *validator.Validate
	logger          *slog.Logger
}

// NewDonationHandler creates a new donation handler
func NewDonationHandler(
	donationService *service.DonationService,
	validator *validator.Validate,
	logger *slog.Logger,
) *DonationHandler {
	return &DonationHandler{
		donationService: donationService,
		validator:       validator,
		logger:          logger,
	}
}

// Create handles POST /api/v1/donations
func (h *DonationHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateDonationRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid request body")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		validationErrors := err.(validator.ValidationErrors)
		h.respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", formatValidationErrors(validationErrors))
		return
	}

	// Map payment method string to payment.Method
	paymentMethod := payment.Method(req.PaymentMethod)

	// Create donation
	result, err := h.donationService.CreateDonation(r.Context(), service.CreateDonationParams{
		DonorName:     req.DonorName,
		DonorEmail:    req.DonorEmail,
		Message:       req.Message,
		Amount:        req.Amount,
		PaymentMethod: paymentMethod,
	})
	if err != nil {
		h.logger.Error("failed to create donation", "error", err)
		h.respondError(w, http.StatusInternalServerError, "CREATE_FAILED", err.Error())
		return
	}

	// Build response
	response := dto.DonationResponse{
		ID:          result.Donation.ID,
		DonorName:   result.Donation.DonorName,
		DonorEmail:  result.Donation.DonorEmail,
		Message:     result.Donation.Message,
		Amount:      result.Donation.Amount,
		Status:      string(result.Donation.Status),
		CreatedAt:   result.Donation.CreatedAt,
		PaymentInfo: h.buildPaymentInfo(result.Payment),
	}

	h.respondJSON(w, http.StatusCreated, response)
}

// GetByID handles GET /api/v1/donations/{id}
func (h *DonationHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid donation ID")
		return
	}

	don, pay, err := h.donationService.GetDonationWithPayment(r.Context(), id)
	if err != nil {
		h.respondError(w, http.StatusNotFound, "NOT_FOUND", "Donation not found")
		return
	}

	response := dto.DonationResponse{
		ID:          don.ID,
		DonorName:   don.DonorName,
		DonorEmail:  don.DonorEmail,
		Message:     don.Message,
		Amount:      don.Amount,
		Status:      string(don.Status),
		CreatedAt:   don.CreatedAt,
		PaidAt:      don.PaidAt,
		PaymentInfo: h.buildPaymentInfo(pay),
	}

	h.respondJSON(w, http.StatusOK, response)
}

// GetStatus handles GET /api/v1/donations/{id}/status
func (h *DonationHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid donation ID")
		return
	}

	don, pay, err := h.donationService.GetDonationWithPayment(r.Context(), id)
	if err != nil {
		h.respondError(w, http.StatusNotFound, "NOT_FOUND", "Donation not found")
		return
	}

	response := map[string]interface{}{
		"donation_id": don.ID,
		"status":      don.Status,
		"paid_at":     don.PaidAt,
	}

	if pay != nil {
		response["payment_status"] = pay.Status
		response["payment_method"] = pay.Method
		response["expires_at"] = pay.ExpiresAt
	}

	h.respondJSON(w, http.StatusOK, response)
}

// List handles GET /api/v1/donations (admin)
func (h *DonationHandler) List(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	page := parseInt(r.URL.Query().Get("page"), 1)
	limit := parseInt(r.URL.Query().Get("limit"), 20)
	status := r.URL.Query().Get("status")

	if limit > 100 {
		limit = 100
	}

	params := donation.ListDonationsParams{
		Page:  page,
		Limit: limit,
	}

	if status != "" {
		s := donation.Status(status)
		params.Status = &s
	}

	result, err := h.donationService.ListDonations(r.Context(), params)
	if err != nil {
		h.logger.Error("failed to list donations", "error", err)
		h.respondError(w, http.StatusInternalServerError, "LIST_FAILED", "Failed to list donations")
		return
	}

	// Build response
	donations := make([]dto.DonationResponse, len(result.Donations))
	for i, don := range result.Donations {
		donations[i] = dto.DonationResponse{
			ID:         don.ID,
			DonorName:  don.DonorName,
			DonorEmail: don.DonorEmail,
			Message:    don.Message,
			Amount:     don.Amount,
			Status:     string(don.Status),
			CreatedAt:  don.CreatedAt,
			PaidAt:     don.PaidAt,
		}
	}

	response := map[string]interface{}{
		"donations": donations,
		"total":     result.Total,
		"page":      result.Page,
		"limit":     result.Limit,
		"pages":     (result.Total + int64(result.Limit) - 1) / int64(result.Limit),
	}

	h.respondJSON(w, http.StatusOK, response)
}

// GetStats handles GET /api/v1/donations/stats (admin)
func (h *DonationHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	// Parse date range
	startStr := r.URL.Query().Get("start_date")
	endStr := r.URL.Query().Get("end_date")

	var startDate, endDate time.Time
	var err error

	if startStr == "" {
		// Default to current month
		now := time.Now()
		startDate = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
	} else {
		startDate, err = time.Parse("2006-01-02", startStr)
		if err != nil {
			h.respondError(w, http.StatusBadRequest, "INVALID_DATE", "Invalid start_date format (use YYYY-MM-DD)")
			return
		}
	}

	if endStr == "" {
		endDate = time.Now()
	} else {
		endDate, err = time.Parse("2006-01-02", endStr)
		if err != nil {
			h.respondError(w, http.StatusBadRequest, "INVALID_DATE", "Invalid end_date format (use YYYY-MM-DD)")
			return
		}
		// Include the entire end day
		endDate = endDate.Add(24*time.Hour - time.Second)
	}

	stats, err := h.donationService.GetDonationStats(r.Context(), startDate, endDate)
	if err != nil {
		h.logger.Error("failed to get donation stats", "error", err)
		h.respondError(w, http.StatusInternalServerError, "STATS_FAILED", "Failed to get donation stats")
		return
	}

	response := map[string]interface{}{
		"total_donations":     stats.TotalDonations,
		"total_amount":        stats.TotalAmount,
		"completed_donations": stats.CompletedDonations,
		"completed_amount":    stats.CompletedAmount,
		"average_amount":      stats.AverageAmount,
		"start_date":          startDate.Format("2006-01-02"),
		"end_date":            endDate.Format("2006-01-02"),
	}

	h.respondJSON(w, http.StatusOK, response)
}

// buildPaymentInfo builds payment info for response
func (h *DonationHandler) buildPaymentInfo(pay *payment.Payment) *dto.PaymentInfo {
	if pay == nil {
		return nil
	}

	return &dto.PaymentInfo{
		PaymentID:     pay.ID,
		Provider:      string(pay.Provider),
		Method:        string(pay.Method),
		Status:        string(pay.Status),
		QRCodeURL:     pay.QRCodeURL,
		VANumber:      pay.VANumber,
		DeepLink:      pay.DeepLink,
		ExpiresAt:     pay.ExpiresAt,
		PaymentPageURL: buildPaymentPageURL(pay),
	}
}

// buildPaymentPageURL builds payment page URL for redirect
func buildPaymentPageURL(pay *payment.Payment) string {
	if pay.QRCodeURL != "" {
		return pay.QRCodeURL
	}
	if pay.DeepLink != "" {
		return pay.DeepLink
	}
	return ""
}

// respondJSON sends JSON response
func (h *DonationHandler) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// respondError sends error response
func (h *DonationHandler) respondError(w http.ResponseWriter, status int, code, message string) {
	h.respondJSON(w, status, dto.ErrorResponse{
		Error:   code,
		Message: message,
	})
}

// formatValidationErrors formats validation errors
func formatValidationErrors(errs validator.ValidationErrors) string {
	if len(errs) == 0 {
		return "Validation failed"
	}

	messages := make([]string, len(errs))
	for i, err := range errs {
		switch err.Tag() {
		case "required":
			messages[i] = err.Field() + " is required"
		case "email":
			messages[i] = err.Field() + " must be a valid email"
		case "min":
			messages[i] = err.Field() + " is too short"
		case "max":
			messages[i] = err.Field() + " is too long"
		case "gte":
			messages[i] = err.Field() + " is below minimum"
		case "lte":
			messages[i] = err.Field() + " is above maximum"
		default:
			messages[i] = err.Field() + " is invalid"
		}
	}

	return messages[0] // Return first error for simplicity
}

// parseInt parses int with default value
func parseInt(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}

	var val int
	if _, err := fmt.Sscanf(s, "%d", &val); err != nil {
		return defaultVal
	}

	return val
}
