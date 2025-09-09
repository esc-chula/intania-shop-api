ALTER TABLE users
    DROP COLUMN IF EXISTS google_sub,
    DROP COLUMN IF EXISTS google_picture,
    DROP COLUMN IF EXISTS email_verified;

-- Note: oauth_accounts and oauth_provider remain dropped.

