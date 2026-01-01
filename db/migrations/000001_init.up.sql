-- migrations/000001_init.up.sql
-- ReveeGate Database Schema

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Donations table
CREATE TABLE donations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    donor_name VARCHAR(100) NOT NULL DEFAULT 'Anonymous',
    donor_email VARCHAR(255),
    message TEXT,
    amount BIGINT NOT NULL CHECK (amount >= 5000),
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    metadata JSONB DEFAULT '{}',
    paid_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Donations indexes
CREATE INDEX idx_donations_status ON donations(status);
CREATE INDEX idx_donations_created_at ON donations(created_at DESC);
CREATE INDEX idx_donations_paid_at ON donations(paid_at DESC) WHERE paid_at IS NOT NULL;
CREATE INDEX idx_donations_status_created ON donations(status, created_at DESC);

-- Payments table
CREATE TABLE payments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    donation_id UUID NOT NULL REFERENCES donations(id) ON DELETE CASCADE,
    provider VARCHAR(50) NOT NULL,
    external_id VARCHAR(255) NOT NULL,
    payment_method VARCHAR(50) NOT NULL,
    amount BIGINT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    qr_code_url TEXT,
    va_number VARCHAR(100),
    deep_link TEXT,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    paid_at TIMESTAMP WITH TIME ZONE,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(provider, external_id)
);

-- Payments indexes
CREATE INDEX idx_payments_donation_id ON payments(donation_id);
CREATE INDEX idx_payments_external_id ON payments(provider, external_id);
CREATE INDEX idx_payments_status ON payments(status);
CREATE INDEX idx_payments_expires_at ON payments(expires_at) WHERE status = 'pending';
CREATE INDEX idx_payments_status_expires ON payments(status, expires_at) WHERE status = 'pending';

-- Webhook logs table (for debugging & reconciliation)
CREATE TABLE webhook_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    provider VARCHAR(50) NOT NULL,
    event_type VARCHAR(100),
    external_id VARCHAR(255),
    status_code INTEGER,
    raw_payload JSONB NOT NULL,
    signature VARCHAR(512),
    ip_address INET,
    processed BOOLEAN DEFAULT FALSE,
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Webhook logs indexes
CREATE INDEX idx_webhook_logs_provider ON webhook_logs(provider);
CREATE INDEX idx_webhook_logs_external_id ON webhook_logs(external_id);
CREATE INDEX idx_webhook_logs_created_at ON webhook_logs(created_at DESC);
CREATE INDEX idx_webhook_logs_processed ON webhook_logs(processed) WHERE NOT processed;

-- Admin users table
CREATE TABLE admin_users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    username VARCHAR(100) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL UNIQUE,
    is_active BOOLEAN DEFAULT TRUE,
    last_login_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Admin users indexes
CREATE INDEX idx_admin_users_username ON admin_users(username) WHERE is_active = TRUE;
CREATE INDEX idx_admin_users_email ON admin_users(email) WHERE is_active = TRUE;

-- Overlay tokens table
CREATE TABLE overlay_tokens (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    token VARCHAR(64) NOT NULL UNIQUE,
    description VARCHAR(255),
    is_active BOOLEAN DEFAULT TRUE,
    last_used_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Overlay tokens indexes
CREATE INDEX idx_overlay_tokens_token ON overlay_tokens(token) WHERE is_active = TRUE;

-- Audit logs table
CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID REFERENCES admin_users(id),
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(50),
    resource_id UUID,
    changes JSONB,
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Audit logs indexes
CREATE INDEX idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at DESC);
CREATE INDEX idx_audit_logs_action ON audit_logs(action);
CREATE INDEX idx_audit_logs_resource ON audit_logs(resource_type, resource_id);

-- Sessions table (for admin dashboard)
CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
    token_hash VARCHAR(64) NOT NULL UNIQUE,
    refresh_token_hash VARCHAR(64) UNIQUE,
    ip_address INET,
    user_agent TEXT,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Sessions indexes
CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_token_hash ON sessions(token_hash);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);

-- Updated at trigger function
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Apply triggers
CREATE TRIGGER update_donations_updated_at 
    BEFORE UPDATE ON donations
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_payments_updated_at 
    BEFORE UPDATE ON payments
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_admin_users_updated_at 
    BEFORE UPDATE ON admin_users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Comments for documentation
COMMENT ON TABLE donations IS 'Stores donation records from viewers';
COMMENT ON TABLE payments IS 'Stores payment details linked to donations';
COMMENT ON TABLE webhook_logs IS 'Stores webhook logs from payment providers for debugging and reconciliation';
COMMENT ON TABLE admin_users IS 'Stores admin user accounts for dashboard access';
COMMENT ON TABLE overlay_tokens IS 'Stores tokens for OBS overlay WebSocket authentication';
COMMENT ON TABLE audit_logs IS 'Stores audit trail of admin actions';
COMMENT ON TABLE sessions IS 'Stores admin user sessions';

COMMENT ON COLUMN donations.amount IS 'Amount in Indonesian Rupiah (IDR), minimum 5000';
COMMENT ON COLUMN donations.status IS 'pending, completed, expired, failed, cancelled';
COMMENT ON COLUMN payments.status IS 'pending, paid, expired, failed, refunded';
COMMENT ON COLUMN payments.provider IS 'midtrans, xendit';
COMMENT ON COLUMN payments.payment_method IS 'qris, gopay, dana, ovo, shopeepay, va_bca, etc.';
