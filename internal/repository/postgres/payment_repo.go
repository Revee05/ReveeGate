package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/reveegate/reveegate/internal/domain/payment"
)

// PaymentRepository implements payment.Repository using PostgreSQL
type PaymentRepository struct {
	pool *pgxpool.Pool
}

// NewPaymentRepository creates a new payment repository
func NewPaymentRepository(pool *pgxpool.Pool) *PaymentRepository {
	return &PaymentRepository{pool: pool}
}

// Create creates a new payment
func (r *PaymentRepository) Create(ctx context.Context, p *payment.Payment) error {
	metadata, err := json.Marshal(p.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		INSERT INTO payments (
			id, donation_id, provider, external_id, payment_method, amount, status,
			qr_code_url, va_number, deep_link, expires_at, metadata, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`

	_, err = r.pool.Exec(ctx, query,
		p.ID,
		p.DonationID,
		string(p.Provider),
		p.ExternalID,
		string(p.PaymentMethod),
		p.Amount,
		string(p.Status),
		p.QRCodeURL,
		p.VANumber,
		p.DeepLink,
		p.ExpiresAt,
		metadata,
		p.CreatedAt,
		p.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create payment: %w", err)
	}

	return nil
}

// GetByID gets a payment by ID
func (r *PaymentRepository) GetByID(ctx context.Context, id uuid.UUID) (*payment.Payment, error) {
	query := `
		SELECT id, donation_id, provider, external_id, payment_method, amount, status,
		       qr_code_url, va_number, deep_link, expires_at, paid_at, metadata, created_at, updated_at
		FROM payments
		WHERE id = $1
	`

	row := r.pool.QueryRow(ctx, query, id)
	return r.scanPayment(row)
}

// GetByDonationID gets a payment by donation ID
func (r *PaymentRepository) GetByDonationID(ctx context.Context, donationID uuid.UUID) (*payment.Payment, error) {
	query := `
		SELECT id, donation_id, provider, external_id, payment_method, amount, status,
		       qr_code_url, va_number, deep_link, expires_at, paid_at, metadata, created_at, updated_at
		FROM payments
		WHERE donation_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`

	row := r.pool.QueryRow(ctx, query, donationID)
	return r.scanPayment(row)
}

// GetByExternalID gets a payment by provider and external ID
func (r *PaymentRepository) GetByExternalID(ctx context.Context, provider payment.Provider, externalID string) (*payment.Payment, error) {
	query := `
		SELECT id, donation_id, provider, external_id, payment_method, amount, status,
		       qr_code_url, va_number, deep_link, expires_at, paid_at, metadata, created_at, updated_at
		FROM payments
		WHERE provider = $1 AND external_id = $2
	`

	row := r.pool.QueryRow(ctx, query, string(provider), externalID)
	return r.scanPayment(row)
}

// Update updates a payment
func (r *PaymentRepository) Update(ctx context.Context, p *payment.Payment) error {
	metadata, err := json.Marshal(p.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		UPDATE payments
		SET external_id = $2, status = $3, qr_code_url = $4, va_number = $5,
		    deep_link = $6, paid_at = $7, metadata = $8, updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.pool.Exec(ctx, query,
		p.ID,
		p.ExternalID,
		string(p.Status),
		p.QRCodeURL,
		p.VANumber,
		p.DeepLink,
		p.PaidAt,
		metadata,
	)

	if err != nil {
		return fmt.Errorf("failed to update payment: %w", err)
	}

	if result.RowsAffected() == 0 {
		return errors.New("payment not found")
	}

	return nil
}

// UpdateStatus updates the status of a payment
func (r *PaymentRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status payment.Status) error {
	query := `UPDATE payments SET status = $2, updated_at = NOW() WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id, string(status))
	if err != nil {
		return fmt.Errorf("failed to update payment status: %w", err)
	}

	if result.RowsAffected() == 0 {
		return errors.New("payment not found")
	}

	return nil
}

// GetPendingExpired gets pending payments that have expired
func (r *PaymentRepository) GetPendingExpired(ctx context.Context) ([]*payment.Payment, error) {
	query := `
		SELECT id, donation_id, provider, external_id, payment_method, amount, status,
		       qr_code_url, va_number, deep_link, expires_at, paid_at, metadata, created_at, updated_at
		FROM payments
		WHERE status = 'pending' AND expires_at < NOW()
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending expired payments: %w", err)
	}
	defer rows.Close()

	payments := make([]*payment.Payment, 0)
	for rows.Next() {
		p, err := r.scanPaymentFromRows(rows)
		if err != nil {
			return nil, err
		}
		payments = append(payments, p)
	}

	return payments, nil
}

// List lists payments with filtering and pagination
func (r *PaymentRepository) List(ctx context.Context, params payment.ListPaymentsParams) (*payment.ListPaymentsResult, error) {
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
		SELECT id, donation_id, provider, external_id, payment_method, amount, status,
		       qr_code_url, va_number, deep_link, expires_at, paid_at, metadata, created_at, updated_at
		FROM payments
		WHERE 1=1
	`
	countQuery := `SELECT COUNT(*) FROM payments WHERE 1=1`
	args := []interface{}{}
	argCount := 1

	if params.Status != nil {
		query += fmt.Sprintf(" AND status = $%d", argCount)
		countQuery += fmt.Sprintf(" AND status = $%d", argCount)
		args = append(args, string(*params.Status))
		argCount++
	}

	if params.Provider != nil {
		query += fmt.Sprintf(" AND provider = $%d", argCount)
		countQuery += fmt.Sprintf(" AND provider = $%d", argCount)
		args = append(args, string(*params.Provider))
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
		return nil, fmt.Errorf("failed to count payments: %w", err)
	}

	// Add pagination
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argCount, argCount+1)
	args = append(args, params.Limit, offset)

	// Execute query
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list payments: %w", err)
	}
	defer rows.Close()

	payments := make([]*payment.Payment, 0)
	for rows.Next() {
		p, err := r.scanPaymentFromRows(rows)
		if err != nil {
			return nil, err
		}
		payments = append(payments, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating payments: %w", err)
	}

	totalPages := int(total) / params.Limit
	if int(total)%params.Limit > 0 {
		totalPages++
	}

	return &payment.ListPaymentsResult{
		Payments:   payments,
		Total:      total,
		Page:       params.Page,
		Limit:      params.Limit,
		TotalPages: totalPages,
	}, nil
}

// Helper function to scan a payment from a row
func (r *PaymentRepository) scanPayment(row pgx.Row) (*payment.Payment, error) {
	var p payment.Payment
	var providerStr, methodStr, statusStr string
	var metadataBytes []byte

	err := row.Scan(
		&p.ID,
		&p.DonationID,
		&providerStr,
		&p.ExternalID,
		&methodStr,
		&p.Amount,
		&statusStr,
		&p.QRCodeURL,
		&p.VANumber,
		&p.DeepLink,
		&p.ExpiresAt,
		&p.PaidAt,
		&metadataBytes,
		&p.CreatedAt,
		&p.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("payment not found")
		}
		return nil, fmt.Errorf("failed to scan payment: %w", err)
	}

	p.Provider = payment.Provider(providerStr)
	p.PaymentMethod = payment.Method(methodStr)
	p.Status = payment.Status(statusStr)

	if len(metadataBytes) > 0 {
		if err := json.Unmarshal(metadataBytes, &p.Metadata); err != nil {
			p.Metadata = make(map[string]interface{})
		}
	} else {
		p.Metadata = make(map[string]interface{})
	}

	return &p, nil
}

// Helper function to scan a payment from rows
func (r *PaymentRepository) scanPaymentFromRows(rows pgx.Rows) (*payment.Payment, error) {
	var p payment.Payment
	var providerStr, methodStr, statusStr string
	var metadataBytes []byte

	err := rows.Scan(
		&p.ID,
		&p.DonationID,
		&providerStr,
		&p.ExternalID,
		&methodStr,
		&p.Amount,
		&statusStr,
		&p.QRCodeURL,
		&p.VANumber,
		&p.DeepLink,
		&p.ExpiresAt,
		&p.PaidAt,
		&metadataBytes,
		&p.CreatedAt,
		&p.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to scan payment: %w", err)
	}

	p.Provider = payment.Provider(providerStr)
	p.PaymentMethod = payment.Method(methodStr)
	p.Status = payment.Status(statusStr)

	if len(metadataBytes) > 0 {
		if err := json.Unmarshal(metadataBytes, &p.Metadata); err != nil {
			p.Metadata = make(map[string]interface{})
		}
	} else {
		p.Metadata = make(map[string]interface{})
	}

	return &p, nil
}

// WebhookLogRepository implements payment.WebhookLogRepository using PostgreSQL
type WebhookLogRepository struct {
	pool *pgxpool.Pool
}

// NewWebhookLogRepository creates a new webhook log repository
func NewWebhookLogRepository(pool *pgxpool.Pool) *WebhookLogRepository {
	return &WebhookLogRepository{pool: pool}
}

// Create creates a new webhook log
func (r *WebhookLogRepository) Create(ctx context.Context, log *payment.WebhookLog) error {
	payload, err := json.Marshal(log.RawPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	query := `
		INSERT INTO webhook_logs (id, provider, event_type, external_id, raw_payload, signature, ip_address, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err = r.pool.Exec(ctx, query,
		log.ID,
		string(log.Provider),
		log.EventType,
		log.ExternalID,
		payload,
		log.Signature,
		log.IPAddress,
		log.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create webhook log: %w", err)
	}

	return nil
}

// GetByID gets a webhook log by ID
func (r *WebhookLogRepository) GetByID(ctx context.Context, id uuid.UUID) (*payment.WebhookLog, error) {
	query := `
		SELECT id, provider, event_type, external_id, status_code, raw_payload, signature, 
		       ip_address, processed, error_message, created_at
		FROM webhook_logs
		WHERE id = $1
	`

	var log payment.WebhookLog
	var providerStr string
	var payloadBytes []byte
	var statusCode *int

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&log.ID,
		&providerStr,
		&log.EventType,
		&log.ExternalID,
		&statusCode,
		&payloadBytes,
		&log.Signature,
		&log.IPAddress,
		&log.Processed,
		&log.ErrorMessage,
		&log.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("webhook log not found")
		}
		return nil, fmt.Errorf("failed to get webhook log: %w", err)
	}

	log.Provider = payment.Provider(providerStr)
	if statusCode != nil {
		log.StatusCode = *statusCode
	}

	if len(payloadBytes) > 0 {
		if err := json.Unmarshal(payloadBytes, &log.RawPayload); err != nil {
			log.RawPayload = make(map[string]interface{})
		}
	}

	return &log, nil
}

// List lists webhook logs with filtering and pagination
func (r *WebhookLogRepository) List(ctx context.Context, params payment.ListWebhookLogsParams) (*payment.ListWebhookLogsResult, error) {
	// Set defaults
	if params.Page < 1 {
		params.Page = 1
	}
	if params.Limit < 1 || params.Limit > 100 {
		params.Limit = 50
	}

	offset := (params.Page - 1) * params.Limit

	// Build query
	query := `
		SELECT id, provider, event_type, external_id, status_code, raw_payload, signature,
		       ip_address, processed, error_message, created_at
		FROM webhook_logs
		WHERE 1=1
	`
	countQuery := `SELECT COUNT(*) FROM webhook_logs WHERE 1=1`
	args := []interface{}{}
	argCount := 1

	if params.Provider != nil {
		query += fmt.Sprintf(" AND provider = $%d", argCount)
		countQuery += fmt.Sprintf(" AND provider = $%d", argCount)
		args = append(args, string(*params.Provider))
		argCount++
	}

	if params.Processed != nil {
		query += fmt.Sprintf(" AND processed = $%d", argCount)
		countQuery += fmt.Sprintf(" AND processed = $%d", argCount)
		args = append(args, *params.Processed)
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
		return nil, fmt.Errorf("failed to count webhook logs: %w", err)
	}

	// Add pagination
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argCount, argCount+1)
	args = append(args, params.Limit, offset)

	// Execute query
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list webhook logs: %w", err)
	}
	defer rows.Close()

	logs := make([]*payment.WebhookLog, 0)
	for rows.Next() {
		var log payment.WebhookLog
		var providerStr string
		var payloadBytes []byte
		var statusCode *int

		err := rows.Scan(
			&log.ID,
			&providerStr,
			&log.EventType,
			&log.ExternalID,
			&statusCode,
			&payloadBytes,
			&log.Signature,
			&log.IPAddress,
			&log.Processed,
			&log.ErrorMessage,
			&log.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan webhook log: %w", err)
		}

		log.Provider = payment.Provider(providerStr)
		if statusCode != nil {
			log.StatusCode = *statusCode
		}

		if len(payloadBytes) > 0 {
			if err := json.Unmarshal(payloadBytes, &log.RawPayload); err != nil {
				log.RawPayload = make(map[string]interface{})
			}
		}

		logs = append(logs, &log)
	}

	totalPages := int(total) / params.Limit
	if int(total)%params.Limit > 0 {
		totalPages++
	}

	return &payment.ListWebhookLogsResult{
		Logs:       logs,
		Total:      total,
		Page:       params.Page,
		Limit:      params.Limit,
		TotalPages: totalPages,
	}, nil
}

// MarkAsProcessed marks a webhook log as processed
func (r *WebhookLogRepository) MarkAsProcessed(ctx context.Context, id uuid.UUID, statusCode int, errorMsg string) error {
	query := `UPDATE webhook_logs SET processed = TRUE, status_code = $2, error_message = $3 WHERE id = $1`

	_, err := r.pool.Exec(ctx, query, id, statusCode, errorMsg)
	if err != nil {
		return fmt.Errorf("failed to mark webhook log as processed: %w", err)
	}

	return nil
}
