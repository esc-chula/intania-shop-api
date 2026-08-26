-- +goose Up
CREATE TYPE user_role AS ENUM ('USER', 'ADMIN');
CREATE TYPE product_status AS ENUM ('PREORDER', 'IN_STOCK', 'OUT_OF_STOCK');
CREATE TYPE stock_transaction_type AS ENUM ('INCREMENT', 'DECREMENT', 'ADJUSTMENT', 'INITIAL', 'ORDER', 'RETURN');
CREATE TYPE product_type AS ENUM ('SINGLE', 'MULTIPLE');

CREATE TABLE users (
    user_id BIGSERIAL PRIMARY KEY,
    full_name VARCHAR(100),
    email VARCHAR(100) UNIQUE NOT NULL,
    role user_role NOT NULL DEFAULT 'USER',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    google_id TEXT UNIQUE,
    profile_picture TEXT
);

CREATE TABLE products (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(150) NOT NULL,
    description TEXT,
    price NUMERIC(10, 2) NOT NULL CHECK (price >= 0),
    status product_status NOT NULL,
    category VARCHAR(100),
    stock_quantity INTEGER CHECK (stock_quantity IS NULL OR stock_quantity >= 0),
    images TEXT[] NOT NULL DEFAULT '{}',
    product_type product_type NOT NULL DEFAULT 'SINGLE',
    sku VARCHAR(50),
    product_code VARCHAR(50),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE variants (
    variant_id BIGSERIAL PRIMARY KEY,
    product_id BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    size VARCHAR(20),
    color VARCHAR(50),
    stock_quantity INTEGER CHECK (stock_quantity IS NULL OR stock_quantity >= 0),
    price NUMERIC(10, 2) CHECK (price IS NULL OR price >= 0)
);

CREATE TABLE stock_transactions (
    transaction_id BIGSERIAL PRIMARY KEY,
    product_id BIGINT REFERENCES products(id) ON DELETE CASCADE,
    variant_id BIGINT REFERENCES variants(variant_id) ON DELETE CASCADE,
    transaction_type stock_transaction_type NOT NULL,
    quantity_change INTEGER NOT NULL CHECK (quantity_change <> 0),
    quantity_before INTEGER NOT NULL CHECK (quantity_before >= 0),
    quantity_after INTEGER NOT NULL CHECK (quantity_after >= 0),
    reason VARCHAR(255),
    notes TEXT,
    reference_type VARCHAR(50),
    reference_id BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by VARCHAR(100),
    confirmation_image TEXT,
    CONSTRAINT chk_product_or_variant CHECK (product_id IS NOT NULL OR variant_id IS NOT NULL)
);

CREATE INDEX idx_products_sku ON products(sku);
CREATE INDEX idx_products_product_code ON products(product_code);
CREATE INDEX idx_variants_product ON variants(product_id);
CREATE INDEX idx_stock_tx_product ON stock_transactions(product_id);
CREATE INDEX idx_stock_tx_variant ON stock_transactions(variant_id);
CREATE INDEX idx_stock_tx_created ON stock_transactions(created_at DESC);
CREATE INDEX idx_stock_tx_reference ON stock_transactions(reference_type, reference_id);

-- +goose Down
DROP TABLE IF EXISTS stock_transactions;
DROP TABLE IF EXISTS variants;
DROP TABLE IF EXISTS products;
DROP TABLE IF EXISTS users;
DROP TYPE IF EXISTS product_type;
DROP TYPE IF EXISTS stock_transaction_type;
DROP TYPE IF EXISTS product_status;
DROP TYPE IF EXISTS user_role;
