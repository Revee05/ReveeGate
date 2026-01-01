package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/reveegate/reveegate/internal/domain/donation"
)

// DonationRepository implements donation.Repository using PostgreSQL
type DonationRepository struct {
	pool *pgxpool.Pool
}

// NewDonationRepository creates a new donation repository
func NewDonationRepository(pool *pgxpool.Pool) *DonationRepository {
	return &DonationRepository{pool: pool}
}

// Create creates a new donation
func (r *DonationRepository) Create(ctx context.Context, d *donation.Donation) error {
	metadata, err := json.Marshal(d.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		INSERT INTO donations (id, donor_name, donor_email, message, amount, status, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err = r.pool.Exec(ctx, query,
		d.ID,
		d.DonorName,
		d.DonorEmail,
		d.Message,
		d.Amount,
		string(d.Status),
		metadata,
		d.CreatedAt,
		d.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create donation: %w", err)
	}

	return nil
}

// GetByID gets a donation by ID
func (r *DonationRepository) GetByID(ctx context.Context, id uuid.UUID) (*donation.Donation, error) {
	query := `
		SELECT id, donor_name, donor_email, message, amount, status, metadata, paid_at, created_at, updated_at
		FROM donations
		WHERE id = $1
	`

	row := r.pool.QueryRow(ctx, query, id)
	return r.scanDonation(row)
}

// Update updates a donation
func (r *DonationRepository) Update(ctx context.Context, d *donation.Donation) error {
	metadata, err := json.Marshal(d.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		UPDATE donations
		SET donor_name = $2, donor_email = $3, message = $4, amount = $5, 
		    status = $6, metadata = $7, paid_at = $8, updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.pool.Exec(ctx, query,
		d.ID,
		d.DonorName,
		d.DonorEmail,
		d.Message,
		d.Amount,
		string(d.Status),
		metadata,
		d.PaidAt,
	)

	if err != nil {
		return fmt.Errorf("failed to update donation: %w", err)
	}

	if result.RowsAffected() == 0 {
		return errors.New("donation not found")
	}

	return nil
}

// List lists donations with filtering and pagination
func (r *DonationRepository) List(ctx context.Context, params donation.ListDonationsParams) (*donation.ListDonationsResult, error) {
	// Set defaults
	if params.Page < 1 {
		params.Page = 1
	}
	if params.Limit < 1 || params.Limit > 100 {
		params.Limit = 50
	}

	offset := (params.Page - 1) * params.Limit

	// Build query with filters
	query := `
		SELECT id, donor_name, donor_email, message, amount, status, metadata, paid_at, created_at, updated_at
		FROM donations
		WHERE 1=1
	`
	countQuery := `SELECT COUNT(*) FROM donations WHERE 1=1`
	args := []interface{}{}
	argCount := 1

	if params.Status != nil {
		query += fmt.Sprintf(" AND status = $%d", argCount)
		countQuery += fmt.Sprintf(" AND status = $%d", argCount)
		args = append(args, string(*params.Status))
		argCount++
	}

	if params.StartDate != nil {
		query += fmt.Sprintf(" AND created_at >= $%d", argCount)
		countQuery += fmt.Sprintf(" AND created_at >= $%d", argCount)
		args = append(args, *params.StartDate)
		argCount++
	}

	if params.EndDate != nil {
		query += fmt.Sprintf(" AND created_at <= $%d", argCount)
		countQuery += fmt.Sprintf(" AND created_at <= $%d", argCount)
		args = append(args, *params.EndDate)
		argCount++
	}

	// Get total count
	var total int64
	err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("failed to count donations: %w", err)
	}

	// Add pagination
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argCount, argCount+1)
	args = append(args, params.Limit, offset)

	// Execute query
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list donations: %w", err)
	}
	defer rows.Close()

	donations := make([]*donation.Donation, 0)
	for rows.Next() {
		d, err := r.scanDonationFromRows(rows)
		if err != nil {
			return nil, err
		}
		donations = append(donations, d)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating donations: %w", err)
	}

	totalPages := int(total) / params.Limit
	if int(total)%params.Limit > 0 {
		totalPages++
	}

	return &donation.ListDonationsResult{
		Donations:  donations,
		Total:      total,
		Page:       params.Page,
		Limit:      params.Limit,
		TotalPages: totalPages,
	}, nil
}

// GetPendingExpired gets pending donations that should be expired
func (r *DonationRepository) GetPendingExpired(ctx context.Context, before time.Time) ([]*donation.Donation, error) {
	query := `
		SELECT id, donor_name, donor_email, message, amount, status, metadata, paid_at, created_at, updated_at
		FROM donations
		WHERE status = 'pending' AND created_at < $1
	`

	rows, err := r.pool.Query(ctx, query, before)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending expired donations: %w", err)
	}
	defer rows.Close()

	donations := make([]*donation.Donation, 0)
	for rows.Next() {
		d, err := r.scanDonationFromRows(rows)
		if err != nil {
			return nil, err
		}
		donations = append(donations, d)
	}

	return donations, nil
}

// UpdateStatus updates the status of a donation
func (r *DonationRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status donation.Status) error {
	query := `UPDATE donations SET status = $2, updated_at = NOW() WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id, string(status))
	if err != nil {
		return fmt.Errorf("failed to update donation status: %w", err)
	}

	if result.RowsAffected() == 0 {
		return errors.New("donation not found")
	}

	return nil
}

// GetStats gets donation statistics for a date range
func (r *DonationRepository) GetStats(ctx context.Context, startDate, endDate time.Time) (*donation.DonationStats, error) {
	query := `
		SELECT 
			COUNT(*) as total_donations,
			COALESCE(SUM(amount), 0) as total_amount,
			COALESCE(AVG(amount), 0) as average_amount,
			COUNT(*) FILTER (WHERE status = 'completed') as completed_count,
			COUNT(*) FILTER (WHERE status = 'pending') as pending_count,
			COUNT(*) FILTER (WHERE status = 'failed') as failed_count
		FROM donations
		WHERE created_at >= $1 AND created_at <= $2
	`

	var stats donation.DonationStats
	err := r.pool.QueryRow(ctx, query, startDate, endDate).Scan(
		&stats.TotalDonations,
		&stats.TotalAmount,
		&stats.AverageAmount,
		&stats.CompletedCount,
		&stats.PendingCount,
		&stats.FailedCount,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get donation stats: %w", err)
	}

	return &stats, nil
}

// Helper function to scan a donation from a row
func (r *DonationRepository) scanDonation(row pgx.Row) (*donation.Donation, error) {
	var d donation.Donation
	var statusStr string
	var metadataBytes []byte

	err := row.Scan(
		&d.ID,
		&d.DonorName,
		&d.DonorEmail,
		&d.Message,
		&d.Amount,
		&statusStr,
		&metadataBytes,
		&d.PaidAt,
		&d.CreatedAt,
		&d.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("donation not found")
		}
		return nil, fmt.Errorf("failed to scan donation: %w", err)
	}

	d.Status = donation.Status(statusStr)

	if len(metadataBytes) > 0 {
		if err := json.Unmarshal(metadataBytes, &d.Metadata); err != nil {
			d.Metadata = make(map[string]interface{})
		}
	} else {
		d.Metadata = make(map[string]interface{})
	}

	return &d, nil
}

// Helper function to scan a donation from rows
func (r *DonationRepository) scanDonationFromRows(rows pgx.Rows) (*donation.Donation, error) {
	var d donation.Donation
	var statusStr string
	var metadataBytes []byte

	err := rows.Scan(
		&d.ID,
		&d.DonorName,
		&d.DonorEmail,
		&d.Message,
		&d.Amount,
		&statusStr,
		&metadataBytes,
		&d.PaidAt,
		&d.CreatedAt,
		&d.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to scan donation: %w", err)
	}

	d.Status = donation.Status(statusStr)

	if len(metadataBytes) > 0 {
		if err := json.Unmarshal(metadataBytes, &d.Metadata); err != nil {
			d.Metadata = make(map[string]interface{})
		}
	} else {
		d.Metadata = make(map[string]interface{})
	}

	return &d, nil
}
