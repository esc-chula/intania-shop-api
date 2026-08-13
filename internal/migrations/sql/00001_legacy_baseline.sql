-- +goose Up
CREATE TYPE user_role AS ENUM ('USER', 'ADMIN');
CREATE TYPE product_status AS ENUM ('PREORDER', 'IN_STOCK', 'OUT_OF_STOCK');
CREATE TYPE order_status AS ENUM ('CART', 'PENDING_PAYMENT', 'CONFIRMED', 'SHIPPING', 'COMPLETED');
CREATE TYPE delivery_type AS ENUM ('PICKUP', 'SHIPPING');
CREATE TYPE payment_status AS ENUM ('PENDING', 'VERIFIED', 'REJECTED');
CREATE TYPE stock_transaction_type AS ENUM ('INCREMENT', 'DECREMENT', 'ADJUSTMENT', 'INITIAL', 'ORDER', 'RETURN');
CREATE TYPE payment_type AS ENUM ('REAL_MONEY', 'QR_CODE');
CREATE TYPE product_type AS ENUM ('SINGLE', 'MULTIPLE');

CREATE TABLE users (
    user_id BIGSERIAL PRIMARY KEY,
    full_name VARCHAR(100),
    email VARCHAR(100) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    phone VARCHAR(20),
    role user_role NOT NULL DEFAULT 'USER',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    google_id TEXT,
    profile_picture TEXT
);

CREATE TABLE user_addresses (
    address_id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    address TEXT,
    is_default BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE products (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(150) NOT NULL,
    description TEXT,
    price NUMERIC(10, 2) NOT NULL,
    status product_status NOT NULL,
    category VARCHAR(100),
    stock_quantity INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    images TEXT[],
    preview_video TEXT,
    shipping TEXT[],
    product_type product_type DEFAULT 'SINGLE',
    min_order INTEGER DEFAULT 1,
    max_order INTEGER,
    size_chart TEXT,
    pickup_methods JSONB,
    pickup_location TEXT,
    shipping_fee NUMERIC(10, 2),
    sku VARCHAR(50),
    product_code VARCHAR(50)
);

CREATE TABLE variants (
    variant_id BIGSERIAL PRIMARY KEY,
    product_id BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    size VARCHAR(20),
    color VARCHAR(50),
    stock_quantity INTEGER,
    price NUMERIC(10, 2)
);

CREATE TABLE favorites (
    user_id BIGINT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    product_id BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, product_id)
);

CREATE TABLE cart (
    cart_id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE cart_items (
    item_id BIGSERIAL PRIMARY KEY,
    cart_id BIGINT NOT NULL REFERENCES cart(cart_id) ON DELETE CASCADE,
    variant_id BIGINT NOT NULL REFERENCES variants(variant_id),
    quantity INTEGER
);

CREATE TABLE orders (
    order_id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(user_id),
    total_amount NUMERIC(10, 2),
    order_status order_status NOT NULL DEFAULT 'CART',
    delivery_type delivery_type,
    shipping_address TEXT,
    tracking_number VARCHAR(100),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    order_code VARCHAR(20) NOT NULL,
    delivery_fee NUMERIC(10, 2) NOT NULL DEFAULT 0
);

CREATE TABLE order_items (
    order_item_id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES orders(order_id) ON DELETE CASCADE,
    variant_id BIGINT NOT NULL REFERENCES variants(variant_id),
    quantity INTEGER,
    unit_price NUMERIC(10, 2),
    product_name VARCHAR(150),
    variant_size VARCHAR(20),
    variant_color VARCHAR(50),
    image_url TEXT
);

CREATE TABLE payments (
    payment_id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES orders(order_id) ON DELETE CASCADE,
    amount_paid NUMERIC(10, 2),
    slip_url VARCHAR(255),
    payment_status payment_status NOT NULL DEFAULT 'PENDING',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE promos (
    promo_id BIGSERIAL PRIMARY KEY,
    img_url VARCHAR(500) NOT NULL,
    link_url VARCHAR(500),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE banners (
    banner_id BIGSERIAL PRIMARY KEY,
    img_url VARCHAR(500) NOT NULL,
    link_url VARCHAR(500),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE stock_transactions (
    transaction_id BIGSERIAL PRIMARY KEY,
    product_id BIGINT REFERENCES products(id) ON DELETE CASCADE,
    variant_id BIGINT REFERENCES variants(variant_id) ON DELETE CASCADE,
    transaction_type stock_transaction_type NOT NULL,
    quantity_change INTEGER NOT NULL,
    quantity_before INTEGER NOT NULL,
    quantity_after INTEGER NOT NULL,
    reason VARCHAR(255),
    reference_type VARCHAR(50),
    reference_id BIGINT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by VARCHAR(100),
    gender VARCHAR(20),
    confirmation_image TEXT,
    tip NUMERIC(10, 2),
    payment_type payment_type,
    total_money_receive NUMERIC(10, 2),
    CONSTRAINT chk_product_or_variant CHECK (product_id IS NOT NULL OR variant_id IS NOT NULL)
);

CREATE UNIQUE INDEX orders_order_code_key ON orders(order_code);
CREATE INDEX idx_promos_is_active ON promos(is_active);
CREATE INDEX idx_banners_is_active ON banners(is_active);
CREATE INDEX idx_stock_tx_product ON stock_transactions(product_id);
CREATE INDEX idx_stock_tx_variant ON stock_transactions(variant_id);
CREATE INDEX idx_stock_tx_created ON stock_transactions(created_at DESC);
CREATE INDEX idx_stock_tx_reference ON stock_transactions(reference_type, reference_id);
CREATE INDEX idx_products_sku ON products(sku);
CREATE INDEX idx_products_product_code ON products(product_code);
CREATE INDEX idx_users_google_id ON users(google_id);

-- +goose Down
DO $$
BEGIN
    RAISE EXCEPTION 'the legacy baseline migration is irreversible';
END;
$$;
