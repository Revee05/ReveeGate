package donation

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Status represents the status of a donation
type Status string

const (
	StatusPending   Status = "pending"
	StatusCompleted Status = "completed"
	StatusExpired   Status = "expired"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

// Donation represents a donation entity
type Donation struct {
	ID         uuid.UUID              `json:"id"`
	DonorName  string                 `json:"donor_name"`
	DonorEmail string                 `json:"donor_email,omitempty"`
	Message    string                 `json:"message,omitempty"`
	Amount     int64                  `json:"amount"`
	Status     Status                 `json:"status"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	PaidAt     *time.Time             `json:"paid_at,omitempty"`
	CreatedAt  time.Time              `json:"created_at"`
	UpdatedAt  time.Time              `json:"updated_at"`
}

// NewDonation creates a new donation
func NewDonation(donorName, donorEmail, message string, amount int64) *Donation {
	now := time.Now()

	if donorName == "" {
		donorName = "Anonymous"
	}

	return &Donation{
		ID:         uuid.New(),
		DonorName:  donorName,
		DonorEmail: donorEmail,
		Message:    message,
		Amount:     amount,
		Status:     StatusPending,
		Metadata:   make(map[string]interface{}),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

// MarkAsPaid marks the donation as paid
func (d *Donation) MarkAsPaid() {
	now := time.Now()
	d.Status = StatusCompleted
	d.PaidAt = &now
	d.UpdatedAt = now
}

// MarkAsExpired marks the donation as expired
func (d *Donation) MarkAsExpired() {
	d.Status = StatusExpired
	d.UpdatedAt = time.Now()
}

// MarkAsFailed marks the donation as failed
func (d *Donation) MarkAsFailed() {
	d.Status = StatusFailed
	d.UpdatedAt = time.Now()
}

// IsPending checks if the donation is pending
func (d *Donation) IsPending() bool {
	return d.Status == StatusPending
}

// IsCompleted checks if the donation is completed
func (d *Donation) IsCompleted() bool {
	return d.Status == StatusCompleted
}

// CreateDonationParams holds parameters for creating a donation
type CreateDonationParams struct {
	DonorName     string
	DonorEmail    string
	Message       string
	Amount        int64
	PaymentMethod string
}

// ListDonationsParams holds parameters for listing donations
type ListDonationsParams struct {
	Status    *Status
	StartDate *time.Time
	EndDate   *time.Time
	Page      int
	Limit     int
}

// ListDonationsResult holds the result of listing donations
type ListDonationsResult struct {
	Donations  []*Donation `json:"donations"`
	Total      int64       `json:"total"`
	Page       int         `json:"page"`
	Limit      int         `json:"limit"`
	TotalPages int         `json:"total_pages"`
}

// Repository defines the donation repository interface
type Repository interface {
	Create(ctx context.Context, donation *Donation) error
	GetByID(ctx context.Context, id uuid.UUID) (*Donation, error)
	Update(ctx context.Context, donation *Donation) error
	List(ctx context.Context, params ListDonationsParams) (*ListDonationsResult, error)
	GetPendingExpired(ctx context.Context, before time.Time) ([]*Donation, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status Status) error
	GetStats(ctx context.Context, startDate, endDate time.Time) (*DonationStats, error)
}

// DonationStats holds donation statistics
type DonationStats struct {
	TotalDonations     int64   `json:"total_donations"`
	TotalAmount        int64   `json:"total_amount"`
	AverageAmount      float64 `json:"average_amount"`
	CompletedDonations int64   `json:"completed_donations"`
	CompletedAmount    int64   `json:"completed_amount"`
	CompletedCount     int64   `json:"completed_count"`
	PendingCount       int64   `json:"pending_count"`
	FailedCount        int64   `json:"failed_count"`
}
