-- Enum types
CREATE TYPE user_role AS ENUM ('USER', 'ADMIN');
CREATE TYPE product_status AS ENUM ('PREORDER', 'IN_STOCK', 'OUT_OF_STOCK');
CREATE TYPE order_status AS ENUM ('CART', 'PENDING_PAYMENT', 'CONFIRMED', 'SHIPPING', 'COMPLETED');
CREATE TYPE delivery_type AS ENUM ('PICKUP', 'SHIPPING');
CREATE TYPE payment_status AS ENUM ('PENDING', 'VERIFIED', 'REJECTED');

-- Users
CREATE TABLE users (
    user_id BIGSERIAL PRIMARY KEY,
    full_name VARCHAR(100),
    email VARCHAR(100) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    phone VARCHAR(20),
    role user_role NOT NULL DEFAULT 'USER',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- UserAddresses
CREATE TABLE user_addresses (
    address_id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    address TEXT,
    is_default BOOLEAN NOT NULL DEFAULT FALSE
);

-- Products
CREATE TABLE products (
    product_id BIGSERIAL PRIMARY KEY,
    name VARCHAR(150) NOT NULL,
    description TEXT,
    base_price NUMERIC(10,2) NOT NULL,
    status product_status NOT NULL
);

-- Variants
CREATE TABLE variants (
    variant_id BIGSERIAL PRIMARY KEY,
    product_id BIGINT NOT NULL REFERENCES products(product_id) ON DELETE CASCADE,
    size VARCHAR(20),
    color VARCHAR(50),
    stock_quantity INT
);

-- Favorites
CREATE TABLE favorites (
    user_id BIGINT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    product_id BIGINT NOT NULL REFERENCES products(product_id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, product_id)
);

-- Cart
CREATE TABLE cart (
    cart_id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- CartItems
CREATE TABLE cart_items (
    item_id BIGSERIAL PRIMARY KEY,
    cart_id BIGINT NOT NULL REFERENCES cart(cart_id) ON DELETE CASCADE,
    variant_id BIGINT NOT NULL REFERENCES variants(variant_id),
    quantity INT
);

-- Orders
CREATE TABLE orders (
    order_id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(user_id),
    total_amount NUMERIC(10,2),
    order_status order_status NOT NULL DEFAULT 'CART',
    delivery_type delivery_type,
    shipping_address TEXT,
    tracking_number VARCHAR(100),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- OrderItems
CREATE TABLE order_items (
    order_item_id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES orders(order_id) ON DELETE CASCADE,
    variant_id BIGINT NOT NULL REFERENCES variants(variant_id),
    quantity INT,
    unit_price NUMERIC(10,2)
);

-- Payments
CREATE TABLE payments (
    payment_id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES orders(order_id) ON DELETE CASCADE,
    amount_paid NUMERIC(10,2),
    slip_url VARCHAR(255),
    payment_status payment_status NOT NULL DEFAULT 'PENDING',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

