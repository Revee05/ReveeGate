-- name: CreateDonation :one
INSERT INTO donations (
    id, donor_name, donor_email, message, amount, status, metadata
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
) RETURNING *;

-- name: GetDonationByID :one
SELECT * FROM donations WHERE id = $1;

-- name: UpdateDonation :one
UPDATE donations SET
    donor_name = COALESCE($2, donor_name),
    donor_email = COALESCE($3, donor_email),
    message = COALESCE($4, message),
    status = COALESCE($5, status),
    metadata = COALESCE($6, metadata),
    paid_at = COALESCE($7, paid_at)
WHERE id = $1
RETURNING *;

-- name: UpdateDonationStatus :exec
UPDATE donations SET status = $2 WHERE id = $1;

-- name: UpdateDonationPaid :exec
UPDATE donations SET status = 'completed', paid_at = NOW() WHERE id = $1;

-- name: ListDonations :many
SELECT * FROM donations
WHERE 
    ($1::varchar IS NULL OR status = $1)
    AND ($2::timestamptz IS NULL OR created_at >= $2)
    AND ($3::timestamptz IS NULL OR created_at <= $3)
ORDER BY created_at DESC
LIMIT $4 OFFSET $5;

-- name: CountDonations :one
SELECT COUNT(*) FROM donations
WHERE 
    ($1::varchar IS NULL OR status = $1)
    AND ($2::timestamptz IS NULL OR created_at >= $2)
    AND ($3::timestamptz IS NULL OR created_at <= $3);

-- name: GetPendingExpiredDonations :many
SELECT * FROM donations 
WHERE status = 'pending' AND created_at < $1;

-- name: GetDonationStats :one
SELECT 
    COUNT(*) as total_donations,
    COALESCE(SUM(amount), 0) as total_amount,
    COALESCE(AVG(amount), 0) as average_amount,
    COUNT(*) FILTER (WHERE status = 'completed') as completed_count,
    COUNT(*) FILTER (WHERE status = 'pending') as pending_count,
    COUNT(*) FILTER (WHERE status = 'failed') as failed_count
FROM donations
WHERE created_at >= $1 AND created_at <= $2;

-- name: GetRecentCompletedDonations :many
SELECT * FROM donations 
WHERE status = 'completed' 
ORDER BY paid_at DESC 
LIMIT $1;

-- name: CreatePayment :one
INSERT INTO payments (
    id, donation_id, provider, external_id, payment_method, amount, status,
    qr_code_url, va_number, deep_link, expires_at, metadata
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
) RETURNING *;

-- name: GetPaymentByID :one
SELECT * FROM payments WHERE id = $1;

-- name: GetPaymentByDonationID :one
SELECT * FROM payments WHERE donation_id = $1;

-- name: GetPaymentByExternalID :one
SELECT * FROM payments WHERE provider = $1 AND external_id = $2;

-- name: UpdatePayment :one
UPDATE payments SET
    external_id = COALESCE($2, external_id),
    status = COALESCE($3, status),
    qr_code_url = COALESCE($4, qr_code_url),
    va_number = COALESCE($5, va_number),
    deep_link = COALESCE($6, deep_link),
    paid_at = COALESCE($7, paid_at),
    metadata = COALESCE($8, metadata)
WHERE id = $1
RETURNING *;

-- name: UpdatePaymentStatus :exec
UPDATE payments SET status = $2 WHERE id = $1;

-- name: UpdatePaymentPaid :exec
UPDATE payments SET status = 'paid', paid_at = NOW() WHERE id = $1;

-- name: GetPendingExpiredPayments :many
SELECT * FROM payments 
WHERE status = 'pending' AND expires_at < NOW();

-- name: ListPayments :many
SELECT * FROM payments
WHERE 
    ($1::varchar IS NULL OR status = $1)
    AND ($2::varchar IS NULL OR provider = $2)
    AND ($3::timestamptz IS NULL OR created_at >= $3)
    AND ($4::timestamptz IS NULL OR created_at <= $4)
ORDER BY created_at DESC
LIMIT $5 OFFSET $6;

-- name: CountPayments :one
SELECT COUNT(*) FROM payments
WHERE 
    ($1::varchar IS NULL OR status = $1)
    AND ($2::varchar IS NULL OR provider = $2)
    AND ($3::timestamptz IS NULL OR created_at >= $3)
    AND ($4::timestamptz IS NULL OR created_at <= $4);

-- name: CreateWebhookLog :one
INSERT INTO webhook_logs (
    id, provider, event_type, external_id, raw_payload, signature, ip_address
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
) RETURNING *;

-- name: GetWebhookLogByID :one
SELECT * FROM webhook_logs WHERE id = $1;

-- name: UpdateWebhookLogProcessed :exec
UPDATE webhook_logs SET 
    processed = TRUE, 
    status_code = $2, 
    error_message = $3 
WHERE id = $1;

-- name: ListWebhookLogs :many
SELECT * FROM webhook_logs
WHERE 
    ($1::varchar IS NULL OR provider = $1)
    AND ($2::bool IS NULL OR processed = $2)
    AND ($3::timestamptz IS NULL OR created_at >= $3)
    AND ($4::timestamptz IS NULL OR created_at <= $4)
ORDER BY created_at DESC
LIMIT $5 OFFSET $6;

-- name: CountWebhookLogs :one
SELECT COUNT(*) FROM webhook_logs
WHERE 
    ($1::varchar IS NULL OR provider = $1)
    AND ($2::bool IS NULL OR processed = $2)
    AND ($3::timestamptz IS NULL OR created_at >= $3)
    AND ($4::timestamptz IS NULL OR created_at <= $4);

-- name: CreateAdminUser :one
INSERT INTO admin_users (
    id, username, password_hash, email
) VALUES (
    $1, $2, $3, $4
) RETURNING *;

-- name: GetAdminUserByID :one
SELECT * FROM admin_users WHERE id = $1 AND is_active = TRUE;

-- name: GetAdminUserByUsername :one
SELECT * FROM admin_users WHERE username = $1 AND is_active = TRUE;

-- name: GetAdminUserByEmail :one
SELECT * FROM admin_users WHERE email = $1 AND is_active = TRUE;

-- name: UpdateAdminUserLastLogin :exec
UPDATE admin_users SET last_login_at = NOW() WHERE id = $1;

-- name: UpdateAdminUserPassword :exec
UPDATE admin_users SET password_hash = $2 WHERE id = $1;

-- name: DeactivateAdminUser :exec
UPDATE admin_users SET is_active = FALSE WHERE id = $1;

-- name: CreateOverlayToken :one
INSERT INTO overlay_tokens (
    id, token, description
) VALUES (
    $1, $2, $3
) RETURNING *;

-- name: GetOverlayTokenByToken :one
SELECT * FROM overlay_tokens WHERE token = $1 AND is_active = TRUE;

-- name: UpdateOverlayTokenLastUsed :exec
UPDATE overlay_tokens SET last_used_at = NOW() WHERE token = $1;

-- name: DeactivateOverlayToken :exec
UPDATE overlay_tokens SET is_active = FALSE WHERE id = $1;

-- name: ListOverlayTokens :many
SELECT * FROM overlay_tokens ORDER BY created_at DESC;

-- name: CreateAuditLog :one
INSERT INTO audit_logs (
    id, user_id, action, resource_type, resource_id, changes, ip_address, user_agent
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
) RETURNING *;

-- name: ListAuditLogs :many
SELECT * FROM audit_logs
WHERE 
    ($1::uuid IS NULL OR user_id = $1)
    AND ($2::varchar IS NULL OR action = $2)
    AND ($3::timestamptz IS NULL OR created_at >= $3)
    AND ($4::timestamptz IS NULL OR created_at <= $4)
ORDER BY created_at DESC
LIMIT $5 OFFSET $6;

-- name: CreateSession :one
INSERT INTO sessions (
    id, user_id, token_hash, refresh_token_hash, ip_address, user_agent, expires_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
) RETURNING *;

-- name: GetSessionByTokenHash :one
SELECT * FROM sessions WHERE token_hash = $1 AND expires_at > NOW();

-- name: GetSessionByRefreshTokenHash :one
SELECT * FROM sessions WHERE refresh_token_hash = $1;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE id = $1;

-- name: DeleteSessionsByUserID :exec
DELETE FROM sessions WHERE user_id = $1;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions WHERE expires_at < NOW();
