package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/reveegate/reveegate/internal/domain/donation"
	"github.com/reveegate/reveegate/internal/domain/payment"
	"github.com/reveegate/reveegate/internal/domain/provider"
	redisRepo "github.com/reveegate/reveegate/internal/repository/redis"
)

// DonationService handles donation business logic
type DonationService struct {
	donationRepo   donation.Repository
	paymentRepo    payment.Repository
	webhookLogRepo payment.WebhookLogRepository
	provider       provider.Provider
	pubsub         *redisRepo.PubSub
	cache          *redisRepo.Cache
	logger         *slog.Logger
}

// NewDonationService creates a new donation service
func NewDonationService(
	donationRepo donation.Repository,
	paymentRepo payment.Repository,
	webhookLogRepo payment.WebhookLogRepository,
	prov provider.Provider,
	pubsub *redisRepo.PubSub,
	cache *redisRepo.Cache,
	logger *slog.Logger,
) *DonationService {
	return &DonationService{
		donationRepo:   donationRepo,
		paymentRepo:    paymentRepo,
		webhookLogRepo: webhookLogRepo,
		provider:       prov,
		pubsub:         pubsub,
		cache:          cache,
		logger:         logger,
	}
}

// CreateDonationParams holds parameters for creating a donation
type CreateDonationParams struct {
	DonorName     string
	DonorEmail    string
	Message       string
	Amount        int64
	PaymentMethod payment.Method
}

// CreateDonationResult holds the result of creating a donation
type CreateDonationResult struct {
	Donation *donation.Donation
	Payment  *payment.Payment
}

// CreateDonation creates a new donation with payment
func (s *DonationService) CreateDonation(ctx context.Context, params CreateDonationParams) (*CreateDonationResult, error) {
	// Validate amount
	if params.Amount < 5000 {
		return nil, errors.New("minimum donation amount is Rp 5,000")
	}

	if params.Amount > 100000000 {
		return nil, errors.New("maximum donation amount is Rp 100,000,000")
	}

	// Create donation entity
	don := donation.NewDonation(params.DonorName, params.DonorEmail, params.Message, params.Amount)

	// Save donation to database
	if err := s.donationRepo.Create(ctx, don); err != nil {
		return nil, fmt.Errorf("failed to create donation: %w", err)
	}

	// Set payment expiry (24 hours)
	expiresAt := time.Now().Add(24 * time.Hour)

	// Create payment entity
	pay := payment.NewPayment(don.ID, s.provider.GetName(), params.PaymentMethod, params.Amount, expiresAt)

	// Create payment with provider
	paymentReq := provider.PaymentRequest{
		OrderID:       pay.GenerateOrderID(),
		Amount:        params.Amount,
		PaymentMethod: params.PaymentMethod,
		CustomerName:  params.DonorName,
		CustomerEmail: params.DonorEmail,
		Description:   fmt.Sprintf("Donation from %s", params.DonorName),
		ExpiryTime:    expiresAt,
	}

	paymentResp, err := s.provider.CreatePayment(ctx, paymentReq)
	if err != nil {
		// Update donation status to failed
		don.MarkAsFailed()
		s.donationRepo.Update(ctx, don)
		return nil, fmt.Errorf("failed to create payment: %w", err)
	}

	// Update payment with provider response
	pay.SetExternalID(paymentResp.ExternalID)
	pay.SetPaymentDetails(paymentResp.QRCodeURL, paymentResp.VANumber, paymentResp.DeepLink)
	pay.ExpiresAt = paymentResp.ExpiresAt

	// Save payment to database
	if err := s.paymentRepo.Create(ctx, pay); err != nil {
		return nil, fmt.Errorf("failed to save payment: %w", err)
	}

	s.logger.Info("donation created",
		"donation_id", don.ID,
		"payment_id", pay.ID,
		"amount", params.Amount,
		"payment_method", params.PaymentMethod,
	)

	return &CreateDonationResult{
		Donation: don,
		Payment:  pay,
	}, nil
}

// GetDonation gets a donation by ID
func (s *DonationService) GetDonation(ctx context.Context, id uuid.UUID) (*donation.Donation, error) {
	return s.donationRepo.GetByID(ctx, id)
}

// GetDonationWithPayment gets a donation with its payment
func (s *DonationService) GetDonationWithPayment(ctx context.Context, donationID uuid.UUID) (*donation.Donation, *payment.Payment, error) {
	don, err := s.donationRepo.GetByID(ctx, donationID)
	if err != nil {
		return nil, nil, err
	}

	pay, err := s.paymentRepo.GetByDonationID(ctx, donationID)
	if err != nil {
		return don, nil, nil // Donation exists but no payment
	}

	return don, pay, nil
}

// ListDonations lists donations with filtering
func (s *DonationService) ListDonations(ctx context.Context, params donation.ListDonationsParams) (*donation.ListDonationsResult, error) {
	return s.donationRepo.List(ctx, params)
}

// GetDonationStats gets donation statistics
func (s *DonationService) GetDonationStats(ctx context.Context, startDate, endDate time.Time) (*donation.DonationStats, error) {
	return s.donationRepo.GetStats(ctx, startDate, endDate)
}

// ProcessWebhookParams holds parameters for processing a webhook
type ProcessWebhookParams struct {
	Provider      payment.Provider
	OrderID       string
	TransactionID string
	Status        payment.Status
	PaidAt        time.Time
	RawPayload    []byte
}

// ProcessWebhook processes a payment webhook
func (s *DonationService) ProcessWebhook(ctx context.Context, params ProcessWebhookParams) error {
	// Check idempotency
	idempotencyKey := redisRepo.IdempotencyKey(string(params.Provider), params.OrderID, params.TransactionID)

	// Try to set idempotency key
	set, err := s.cache.SetNX(ctx, idempotencyKey, "processing", 24*time.Hour)
	if err != nil {
		s.logger.Warn("failed to check idempotency", "error", err)
	}

	if !set {
		// Already processed
		s.logger.Info("duplicate webhook ignored",
			"provider", params.Provider,
			"order_id", params.OrderID,
			"transaction_id", params.TransactionID,
		)
		return nil
	}

	// Get payment by external ID
	pay, err := s.paymentRepo.GetByExternalID(ctx, params.Provider, params.OrderID)
	if err != nil {
		return fmt.Errorf("payment not found: %w", err)
	}

	// Skip if already paid
	if pay.IsPaid() {
		s.logger.Info("payment already paid", "payment_id", pay.ID)
		return nil
	}

	// Update payment status based on webhook status
	switch params.Status {
	case payment.StatusPaid:
		pay.MarkAsPaid()

		// Update payment in database
		if err := s.paymentRepo.Update(ctx, pay); err != nil {
			return fmt.Errorf("failed to update payment: %w", err)
		}

		// Get donation
		don, err := s.donationRepo.GetByID(ctx, pay.DonationID)
		if err != nil {
			return fmt.Errorf("donation not found: %w", err)
		}

		// Mark donation as paid
		don.MarkAsPaid()
		if err := s.donationRepo.Update(ctx, don); err != nil {
			return fmt.Errorf("failed to update donation: %w", err)
		}

		// Publish donation event for real-time notification
		event := redisRepo.NewDonationEvent(
			don.ID.String(),
			don.DonorName,
			don.Message,
			don.Amount,
			don.PaidAt.Format(time.RFC3339),
		)

		if err := s.pubsub.PublishDonationEvent(ctx, event); err != nil {
			s.logger.Error("failed to publish donation event", "error", err)
			// Don't return error, payment was successful
		}

		s.logger.Info("payment completed",
			"donation_id", don.ID,
			"payment_id", pay.ID,
			"amount", don.Amount,
		)

	case payment.StatusExpired:
		pay.MarkAsExpired()
		if err := s.paymentRepo.Update(ctx, pay); err != nil {
			return fmt.Errorf("failed to update payment: %w", err)
		}

		// Update donation status
		don, _ := s.donationRepo.GetByID(ctx, pay.DonationID)
		if don != nil {
			don.MarkAsExpired()
			s.donationRepo.Update(ctx, don)
		}

	case payment.StatusFailed:
		pay.MarkAsFailed()
		if err := s.paymentRepo.Update(ctx, pay); err != nil {
			return fmt.Errorf("failed to update payment: %w", err)
		}

		// Update donation status
		don, _ := s.donationRepo.GetByID(ctx, pay.DonationID)
		if don != nil {
			don.MarkAsFailed()
			s.donationRepo.Update(ctx, don)
		}
	}

	return nil
}

// ManualReconcile manually reconciles a payment
func (s *DonationService) ManualReconcile(ctx context.Context, paymentID uuid.UUID, status payment.Status, reason string) error {
	pay, err := s.paymentRepo.GetByID(ctx, paymentID)
	if err != nil {
		return fmt.Errorf("payment not found: %w", err)
	}

	don, err := s.donationRepo.GetByID(ctx, pay.DonationID)
	if err != nil {
		return fmt.Errorf("donation not found: %w", err)
	}

	switch status {
	case payment.StatusPaid:
		pay.MarkAsPaid()
		don.MarkAsPaid()

		// Publish event
		event := redisRepo.NewDonationEvent(
			don.ID.String(),
			don.DonorName,
			don.Message,
			don.Amount,
			don.PaidAt.Format(time.RFC3339),
		)
		s.pubsub.PublishDonationEvent(ctx, event)

	case payment.StatusFailed:
		pay.MarkAsFailed()
		don.MarkAsFailed()
	}

	// Add reason to metadata
	pay.Metadata["reconciliation_reason"] = reason
	pay.Metadata["reconciliation_at"] = time.Now().Format(time.RFC3339)

	if err := s.paymentRepo.Update(ctx, pay); err != nil {
		return fmt.Errorf("failed to update payment: %w", err)
	}

	if err := s.donationRepo.Update(ctx, don); err != nil {
		return fmt.Errorf("failed to update donation: %w", err)
	}

	s.logger.Info("manual reconciliation completed",
		"payment_id", paymentID,
		"status", status,
		"reason", reason,
	)

	return nil
}
