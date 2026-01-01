package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"github.com/reveegate/reveegate/internal/domain/payment"
	"github.com/reveegate/reveegate/internal/http/dto"
	"github.com/reveegate/reveegate/internal/http/middleware"
	"github.com/reveegate/reveegate/internal/repository/postgres"
	"github.com/reveegate/reveegate/internal/service"
)

// AdminHandler handles admin HTTP requests
type AdminHandler struct {
	donationService *service.DonationService
	adminRepo       *postgres.AdminRepository
	authMiddleware  *middleware.Auth
	validator       *validator.Validate
	logger          *slog.Logger
}

// NewAdminHandler creates a new admin handler
func NewAdminHandler(
	donationService *service.DonationService,
	adminRepo *postgres.AdminRepository,
	authMiddleware *middleware.Auth,
	validator *validator.Validate,
	logger *slog.Logger,
) *AdminHandler {
	return &AdminHandler{
		donationService: donationService,
		adminRepo:       adminRepo,
		authMiddleware:  authMiddleware,
		validator:       validator,
		logger:          logger,
	}
}

// Login handles POST /api/v1/admin/login
func (h *AdminHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req dto.AdminLoginRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid request body")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		validationErrors := err.(validator.ValidationErrors)
		h.respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", formatValidationErrors(validationErrors))
		return
	}

	// Find admin user by email or username
	var admin *postgres.AdminUser
	var err error

	if req.Email != "" {
		admin, err = h.adminRepo.FindByEmail(r.Context(), req.Email)
	} else if req.Username != "" {
		admin, err = h.adminRepo.FindByUsername(r.Context(), req.Username)
	} else {
		h.respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "Email or username is required")
		return
	}

	if err != nil {
		if err == postgres.ErrAdminNotFound {
			h.respondError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid email/username or password")
			return
		}
		h.logger.Error("failed to find admin user", "error", err)
		h.respondError(w, http.StatusInternalServerError, "SERVER_ERROR", "Internal server error")
		return
	}

	// Check if admin is active
	if !admin.IsActive {
		h.respondError(w, http.StatusForbidden, "ACCOUNT_INACTIVE", "Your account has been deactivated")
		return
	}

	// Verify password using pgcrypto
	matches, err := h.adminRepo.VerifyPassword(r.Context(), admin.PasswordHash, req.Password)
	if err != nil {
		h.logger.Error("failed to verify password", "error", err)
		h.respondError(w, http.StatusInternalServerError, "SERVER_ERROR", "Internal server error")
		return
	}

	if !matches {
		h.respondError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid email/username or password")
		return
	}

	// Update last login timestamp
	if err := h.adminRepo.UpdateLastLogin(r.Context(), admin.ID); err != nil {
		h.logger.Error("failed to update last login", "error", err)
		// Non-critical error, continue
	}

	// Generate tokens
	accessToken, refreshToken, err := h.authMiddleware.GenerateTokens(admin.Email, "admin")
	if err != nil {
		h.logger.Error("failed to generate tokens", "error", err)
		h.respondError(w, http.StatusInternalServerError, "TOKEN_ERROR", "Failed to generate tokens")
		return
	}

	h.logger.Info("admin login successful",
		"admin_id", admin.ID,
		"username", admin.Username,
		"email", admin.Email,
	)

	h.respondJSON(w, http.StatusOK, dto.AdminLoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    900, // 15 minutes
		TokenType:    "Bearer",
	})
}

// RefreshToken handles POST /api/v1/admin/refresh
func (h *AdminHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token" validate:"required"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid request body")
		return
	}

	// Validate refresh token
	claims, err := h.authMiddleware.ValidateToken(req.RefreshToken)
	if err != nil {
		h.respondError(w, http.StatusUnauthorized, "INVALID_TOKEN", "Invalid or expired refresh token")
		return
	}

	// Generate new tokens
	accessToken, refreshToken, err := h.authMiddleware.GenerateTokens(claims.Subject, claims.Role)
	if err != nil {
		h.logger.Error("failed to generate tokens", "error", err)
		h.respondError(w, http.StatusInternalServerError, "TOKEN_ERROR", "Failed to generate tokens")
		return
	}

	h.respondJSON(w, http.StatusOK, dto.AdminLoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    900,
		TokenType:    "Bearer",
	})
}

// GetDashboard handles GET /api/v1/admin/dashboard
func (h *AdminHandler) GetDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get today's stats
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)

	todayStats, err := h.donationService.GetDonationStats(ctx, todayStart, now)
	if err != nil {
		h.logger.Error("failed to get today stats", "error", err)
	}

	// Get this month's stats
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)

	monthStats, err := h.donationService.GetDonationStats(ctx, monthStart, now)
	if err != nil {
		h.logger.Error("failed to get month stats", "error", err)
	}

	response := map[string]interface{}{
		"today": map[string]interface{}{
			"total_donations":     todayStats.TotalDonations,
			"total_amount":        todayStats.TotalAmount,
			"completed_donations": todayStats.CompletedDonations,
			"completed_amount":    todayStats.CompletedAmount,
			"average_amount":      todayStats.AverageAmount,
		},
		"month": map[string]interface{}{
			"total_donations":     monthStats.TotalDonations,
			"total_amount":        monthStats.TotalAmount,
			"completed_donations": monthStats.CompletedDonations,
			"completed_amount":    monthStats.CompletedAmount,
			"average_amount":      monthStats.AverageAmount,
		},
		"timestamp": now.Format(time.RFC3339),
	}

	h.respondJSON(w, http.StatusOK, response)
}

// ReconcilePayment handles POST /api/v1/admin/reconcile
func (h *AdminHandler) ReconcilePayment(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PaymentID string `json:"payment_id" validate:"required,uuid"`
		Status    string `json:"status" validate:"required,oneof=paid failed"`
		Reason    string `json:"reason" validate:"required,min=10"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid request body")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		validationErrors := err.(validator.ValidationErrors)
		h.respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", formatValidationErrors(validationErrors))
		return
	}

	paymentID, err := uuid.Parse(req.PaymentID)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid payment ID")
		return
	}

	// Get admin user from context
	claims := r.Context().Value(middleware.ClaimsContextKey{}).(*middleware.Claims)

	status := payment.StatusPaid
	if req.Status == "failed" {
		status = payment.StatusFailed
	}

	reason := req.Reason + " (by " + claims.Subject + ")"

	err = h.donationService.ManualReconcile(r.Context(), paymentID, status, reason)
	if err != nil {
		h.logger.Error("failed to reconcile payment",
			"payment_id", paymentID,
			"error", err,
		)
		h.respondError(w, http.StatusInternalServerError, "RECONCILE_FAILED", err.Error())
		return
	}

	h.logger.Info("payment reconciled",
		"payment_id", paymentID,
		"status", req.Status,
		"admin", claims.Subject,
	)

	h.respondJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"message": "Payment reconciled successfully",
	})
}

// GenerateOverlayToken handles POST /api/v1/admin/overlay-token
func (h *AdminHandler) GenerateOverlayToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name      string `json:"name" validate:"required,min=3,max=50"`
		ExpiresIn int    `json:"expires_in"` // days, 0 = never expires
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid request body")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		validationErrors := err.(validator.ValidationErrors)
		h.respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", formatValidationErrors(validationErrors))
		return
	}

	// Generate unique token
	token := uuid.New().String()

	// TODO: Store token in database with expiry

	response := map[string]interface{}{
		"token": token,
		"name":  req.Name,
	}

	if req.ExpiresIn > 0 {
		expiresAt := time.Now().Add(time.Duration(req.ExpiresIn) * 24 * time.Hour)
		response["expires_at"] = expiresAt.Format(time.RFC3339)
	}

	h.logger.Info("overlay token generated",
		"name", req.Name,
		"expires_in_days", req.ExpiresIn,
	)

	h.respondJSON(w, http.StatusCreated, response)
}

// GetWebhookLogs handles GET /api/v1/admin/webhook-logs
func (h *AdminHandler) GetWebhookLogs(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	page := parseInt(r.URL.Query().Get("page"), 1)
	limit := parseInt(r.URL.Query().Get("limit"), 20)
	provider := r.URL.Query().Get("provider")

	if limit > 100 {
		limit = 100
	}

	// TODO: Implement webhook log listing from repository
	// For now, return empty list

	response := map[string]interface{}{
		"logs":  []interface{}{},
		"total": 0,
		"page":  page,
		"limit": limit,
	}

	if provider != "" {
		response["provider"] = provider
	}

	h.respondJSON(w, http.StatusOK, response)
}

// GetSystemHealth handles GET /api/v1/admin/health
func (h *AdminHandler) GetSystemHealth(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement actual health checks for DB, Redis, etc.

	response := dto.HealthResponse{
		Status: "healthy",
		Services: map[string]string{
			"database":  "ok",
			"redis":     "ok",
			"websocket": "ok",
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	h.respondJSON(w, http.StatusOK, response)
}

// respondJSON sends JSON response
func (h *AdminHandler) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// respondError sends error response
func (h *AdminHandler) respondError(w http.ResponseWriter, status int, code, message string) {
	h.respondJSON(w, status, dto.ErrorResponse{
		Error:   code,
		Message: message,
	})
}
