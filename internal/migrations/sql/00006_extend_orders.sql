-- +goose Up
-- Extends the minimal orders table with the immutable POS checkout snapshot
-- (buyer, staff, payment, and applied-promotion data) captured at the time an
-- order was paid. Every snapshot column is populated once at checkout and is
-- never rewritten, so later Product/User/Promotion edits cannot change order
-- history.
CREATE TYPE order_payment_method AS ENUM ('QR_CODE', 'REAL_MONEY');
CREATE TYPE order_buyer_gender AS ENUM ('MALE', 'FEMALE', 'PREFER_NOT_TO_SAY');

ALTER TABLE orders
    ADD COLUMN order_number VARCHAR(100) NOT NULL DEFAULT '',
    ADD COLUMN staff_id BIGINT REFERENCES users(user_id) ON DELETE SET NULL,
    ADD COLUMN staff_full_name VARCHAR(100) NOT NULL DEFAULT '',
    ADD COLUMN staff_email VARCHAR(150) NOT NULL DEFAULT '',
    ADD COLUMN buyer_gender order_buyer_gender NOT NULL DEFAULT 'PREFER_NOT_TO_SAY',
    ADD COLUMN buyer_age INTEGER CHECK (buyer_age IS NULL OR buyer_age >= 0),
    ADD COLUMN buyer_student_alumni_year VARCHAR(50),
    ADD COLUMN payment_method order_payment_method NOT NULL DEFAULT 'REAL_MONEY',
    ADD COLUMN payment_slip_object_key TEXT,
    ADD COLUMN payment_received_amount NUMERIC(18, 2) CHECK (payment_received_amount IS NULL OR payment_received_amount >= 0),
    ADD COLUMN payment_change_amount NUMERIC(18, 2) CHECK (payment_change_amount IS NULL OR payment_change_amount >= 0),
    ADD COLUMN payment_retained_amount NUMERIC(18, 2) CHECK (payment_retained_amount IS NULL OR payment_retained_amount >= 0),
    ADD COLUMN payment_no_change BOOLEAN,
    ADD COLUMN payment_note VARCHAR(500),
    ADD COLUMN subtotal NUMERIC(18, 2) NOT NULL DEFAULT 0 CHECK (subtotal >= 0),
    ADD COLUMN discount NUMERIC(18, 2) NOT NULL DEFAULT 0 CHECK (discount >= 0),
    ADD COLUMN net_total NUMERIC(18, 2) NOT NULL DEFAULT 0 CHECK (net_total >= 0),
    ADD COLUMN promotion_id BIGINT,
    ADD COLUMN promotion_name VARCHAR(150),
    ADD COLUMN promotion_original_bundle_price NUMERIC(18, 2),
    ADD COLUMN promotion_price NUMERIC(18, 2),
    ADD COLUMN promotion_discount NUMERIC(18, 2);

ALTER TABLE orders ALTER COLUMN order_number DROP DEFAULT;
ALTER TABLE orders ALTER COLUMN staff_full_name DROP DEFAULT;
ALTER TABLE orders ALTER COLUMN staff_email DROP DEFAULT;
ALTER TABLE orders ALTER COLUMN buyer_gender DROP DEFAULT;
ALTER TABLE orders ALTER COLUMN payment_method DROP DEFAULT;
ALTER TABLE orders ALTER COLUMN subtotal DROP DEFAULT;
ALTER TABLE orders ALTER COLUMN discount DROP DEFAULT;
ALTER TABLE orders ALTER COLUMN net_total DROP DEFAULT;

CREATE UNIQUE INDEX uq_orders_project_order_number ON orders (project_id, order_number);
CREATE INDEX idx_orders_project_created_at ON orders (project_id, created_at);
CREATE INDEX idx_orders_staff_id ON orders (staff_id);

CREATE TABLE order_items (
    order_item_id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES orders(order_id) ON DELETE CASCADE,
    product_id BIGINT NOT NULL,
    variant_id BIGINT,
    product_name VARCHAR(150) NOT NULL,
    size VARCHAR(20),
    color VARCHAR(50),
    image_url TEXT,
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    unit_price NUMERIC(18, 2) NOT NULL CHECK (unit_price >= 0),
    line_total NUMERIC(18, 2) NOT NULL CHECK (line_total >= 0),
    inventory_transaction_id BIGINT NOT NULL REFERENCES stock_transactions(transaction_id) ON DELETE RESTRICT
);

CREATE INDEX idx_order_items_order ON order_items (order_id);

-- +goose Down
DROP TABLE IF EXISTS order_items;
DROP INDEX IF EXISTS idx_orders_staff_id;
DROP INDEX IF EXISTS idx_orders_project_created_at;
DROP INDEX IF EXISTS uq_orders_project_order_number;

ALTER TABLE orders
    DROP COLUMN IF EXISTS promotion_discount,
    DROP COLUMN IF EXISTS promotion_price,
    DROP COLUMN IF EXISTS promotion_original_bundle_price,
    DROP COLUMN IF EXISTS promotion_name,
    DROP COLUMN IF EXISTS promotion_id,
    DROP COLUMN IF EXISTS net_total,
    DROP COLUMN IF EXISTS discount,
    DROP COLUMN IF EXISTS subtotal,
    DROP COLUMN IF EXISTS payment_note,
    DROP COLUMN IF EXISTS payment_no_change,
    DROP COLUMN IF EXISTS payment_retained_amount,
    DROP COLUMN IF EXISTS payment_change_amount,
    DROP COLUMN IF EXISTS payment_received_amount,
    DROP COLUMN IF EXISTS payment_slip_object_key,
    DROP COLUMN IF EXISTS payment_method,
    DROP COLUMN IF EXISTS buyer_student_alumni_year,
    DROP COLUMN IF EXISTS buyer_age,
    DROP COLUMN IF EXISTS buyer_gender,
    DROP COLUMN IF EXISTS staff_email,
    DROP COLUMN IF EXISTS staff_full_name,
    DROP COLUMN IF EXISTS staff_id,
    DROP COLUMN IF EXISTS order_number;

DROP TYPE IF EXISTS order_buyer_gender;
DROP TYPE IF EXISTS order_payment_method;
