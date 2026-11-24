-- users table (owned by `user` domain))
CREATE TABLE users (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name              VARCHAR(120),                 -- Nullable for future mobile-OTP signups prior to profile setup
    email             VARCHAR(255) UNIQUE,          -- Nullable for future mobile-only signups
    phone_number      VARCHAR(20)  UNIQUE,          -- E.164 format (e.g., '+919876543210'); future-ready
    password_hash     TEXT,                         -- Nullable for future OAuth/OTP-only accounts
    google_id         VARCHAR(255) UNIQUE,          -- Future Google OAuth2 'sub' ID
    github_id         VARCHAR(255) UNIQUE,          -- Future GitHub OAuth2 ID
    role              VARCHAR(32)  NOT NULL DEFAULT 'user',
    is_active         BOOLEAN      NOT NULL DEFAULT TRUE,
    email_verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- Guarantee every user has at least an email or a phone number
    CONSTRAINT check_users_primary_identifier CHECK (email IS NOT NULL OR phone_number IS NOT NULL)
);

-- indexs
CREATE UNIQUE INDEX idx_users_google_id ON users (LOWER(google_id));
CREATE UNIQUE INDEX idx_users_email ON users (LOWER(email));

-- triggers
CREATE TRIGGER set_timestamp_users BEFORE UPDATE ON users FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

-- multi-session / refresh token table (1 user -> many active sessions; owned by `auth` domain)
CREATE TABLE user_sessions (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id            UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    refresh_token_hash VARCHAR(64) NOT NULL UNIQUE, -- SHA-256 hex hash of the opaque refresh token
    user_agent         TEXT,                        -- Device/browser metadata (e.g., "Chrome on macOS")
    ip_address         VARCHAR(45),                 -- Client IPv4/IPv6 for security audit
    expires_at         TIMESTAMPTZ NOT NULL,
    revoked_at         TIMESTAMPTZ,                 -- NULL = active session; set on token rotation or logout
    last_used_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
);

-- index
CREATE INDEX idx_user_sessions_user_id ON user_sessions(user_id);
CREATE INDEX idx_user_sessions_active  ON user_sessions(user_id) WHERE revoked_at IS NULL;

-- triggers
CREATE TRIGGER set_timestamp_user_sessions BEFORE UPDATE ON users FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();
