-- +goose Up
CREATE TABLE oauth_login_codes (
    code_hash BYTEA PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_oauth_login_codes_expires_at ON oauth_login_codes (expires_at);

-- +goose Down
DROP TABLE IF EXISTS oauth_login_codes;
