-- +goose Up
CREATE TYPE pos_order_status AS ENUM ('COMPLETED');
CREATE TYPE pos_payment_method AS ENUM ('QR_CODE', 'REAL_MONEY');
CREATE TYPE pos_buyer_gender AS ENUM ('MALE', 'FEMALE', 'PREFER_NOT_TO_SAY');

-- Payment slips are uploaded before checkout so that a QR order can only
-- reference an object this server stored itself.
CREATE TABLE payment_slips (
    object_key TEXT PRIMARY KEY,
    url TEXT NOT NULL,
    content_type VARCHAR(100) NOT NULL,
    size_bytes BIGINT NOT NULL CHECK (size_bytes > 0),
    -- Attribution only: a slip outlives the account that uploaded it, so
    -- deleting the uploader clears the reference rather than being blocked.
    uploaded_by BIGINT REFERENCES users(user_id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- The 00003 orders table is a placeholder that only carries the project
-- reference. The columns are added rather than the table recreated so existing
-- order rows and their project references survive the migration. Columns a
-- placeholder row cannot answer for stay nullable; checkout always writes them.
ALTER TABLE orders
    ADD COLUMN order_number VARCHAR(100),
    ADD COLUMN status pos_order_status NOT NULL DEFAULT 'COMPLETED',
    ADD COLUMN staff_user_id BIGINT REFERENCES users(user_id) ON DELETE RESTRICT,
    ADD COLUMN staff_full_name VARCHAR(100),
    ADD COLUMN staff_email VARCHAR(100),
    ADD COLUMN buyer_gender pos_buyer_gender,
    ADD COLUMN buyer_age INTEGER CHECK (buyer_age IS NULL OR buyer_age BETWEEN 0 AND 150),
    ADD COLUMN buyer_student_alumni_year VARCHAR(50),
    ADD COLUMN subtotal NUMERIC(18, 2) NOT NULL DEFAULT 0,
    ADD COLUMN discount NUMERIC(18, 2) NOT NULL DEFAULT 0,
    ADD COLUMN net_total NUMERIC(18, 2) NOT NULL DEFAULT 0,
    ADD COLUMN idempotency_key TEXT,
    ADD COLUMN request_fingerprint TEXT;

UPDATE orders SET order_number = 'ORD-' || lpad(order_id::text, 8, '0')
WHERE order_number IS NULL;

ALTER TABLE orders
    ALTER COLUMN order_number SET NOT NULL,
    ALTER COLUMN subtotal DROP DEFAULT,
    ALTER COLUMN discount DROP DEFAULT,
    ALTER COLUMN net_total DROP DEFAULT,
    ADD CONSTRAINT uq_orders_order_number UNIQUE (order_number),
    -- PostgreSQL treats NULLs as distinct, so placeholder rows without a key
    -- do not collide while every checkout key stays unique.
    ADD CONSTRAINT uq_orders_idempotency_key UNIQUE (idempotency_key),
    ADD CONSTRAINT chk_orders_totals CHECK (
        subtotal >= 0 AND discount >= 0 AND net_total >= 0
        AND net_total = subtotal - discount
    ),
    -- Placeholder rows carry no Idempotency-Key and stay exempt. Every row
    -- checkout writes must carry the complete staff and buyer snapshot the
    -- order reader scans into non-nullable fields.
    ADD CONSTRAINT chk_orders_checkout_snapshot CHECK (
        idempotency_key IS NULL
        OR (staff_user_id IS NOT NULL AND staff_full_name IS NOT NULL
            AND staff_email IS NOT NULL AND buyer_gender IS NOT NULL)
    );

-- Item rows keep the name, variant text, image, and price that applied at
-- payment time, so later master data edits cannot rewrite a paid order.
CREATE TABLE order_items (
    order_item_id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES orders(order_id) ON DELETE CASCADE,
    product_id BIGINT NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
    variant_id BIGINT REFERENCES variants(variant_id) ON DELETE RESTRICT,
    product_name VARCHAR(150) NOT NULL,
    size VARCHAR(20),
    color VARCHAR(50),
    image_url TEXT,
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    unit_price NUMERIC(18, 2) NOT NULL CHECK (unit_price >= 0),
    line_total NUMERIC(18, 2) NOT NULL CHECK (line_total >= 0),
    inventory_transaction_id BIGINT NOT NULL
        REFERENCES stock_transactions(transaction_id) ON DELETE RESTRICT,
    CONSTRAINT uq_order_items_transaction UNIQUE (inventory_transaction_id)
);

CREATE UNIQUE INDEX uq_order_items_identity
    ON order_items (order_id, product_id, COALESCE(variant_id, 0));

CREATE TABLE order_payments (
    order_id BIGINT PRIMARY KEY REFERENCES orders(order_id) ON DELETE CASCADE,
    method pos_payment_method NOT NULL,
    slip_object_key TEXT REFERENCES payment_slips(object_key) ON DELETE RESTRICT,
    slip_url TEXT,
    received_amount NUMERIC(18, 2) CHECK (received_amount IS NULL OR received_amount >= 0),
    change_amount NUMERIC(18, 2) CHECK (change_amount IS NULL OR change_amount >= 0),
    retained_amount NUMERIC(18, 2) CHECK (retained_amount IS NULL OR retained_amount >= 0),
    no_change BOOLEAN,
    note VARCHAR(500),
    -- One slip backs at most one order.
    CONSTRAINT uq_order_payments_slip UNIQUE (slip_object_key),
    CONSTRAINT chk_order_payment_shape CHECK (
        (method = 'QR_CODE'
            AND slip_object_key IS NOT NULL AND slip_url IS NOT NULL
            AND received_amount IS NULL AND change_amount IS NULL
            AND retained_amount IS NULL AND no_change IS NULL)
        OR
        (method = 'REAL_MONEY'
            AND slip_object_key IS NULL AND slip_url IS NULL
            AND received_amount IS NOT NULL AND change_amount IS NOT NULL
            AND retained_amount IS NOT NULL AND no_change IS NOT NULL)
    )
);

-- At most one promotion per order, snapshotted without a promotion foreign key
-- so deleting the promotion later cannot alter or block a paid order.
CREATE TABLE order_promotions (
    order_id BIGINT PRIMARY KEY REFERENCES orders(order_id) ON DELETE CASCADE,
    promotion_id BIGINT NOT NULL,
    name VARCHAR(150) NOT NULL,
    original_bundle_price NUMERIC(18, 2) NOT NULL CHECK (original_bundle_price >= 0),
    promotion_price NUMERIC(18, 2) NOT NULL CHECK (promotion_price >= 0),
    discount NUMERIC(18, 2) NOT NULL CHECK (discount >= 0),
    CONSTRAINT chk_order_promotion_discount
        CHECK (discount = original_bundle_price - promotion_price)
);

-- +goose Down
DROP TABLE IF EXISTS order_promotions;
DROP TABLE IF EXISTS order_payments;
DROP INDEX IF EXISTS uq_order_items_identity;
DROP TABLE IF EXISTS order_items;

ALTER TABLE orders
    DROP CONSTRAINT IF EXISTS chk_orders_checkout_snapshot,
    DROP CONSTRAINT IF EXISTS chk_orders_totals,
    DROP CONSTRAINT IF EXISTS uq_orders_idempotency_key,
    DROP CONSTRAINT IF EXISTS uq_orders_order_number,
    DROP COLUMN IF EXISTS request_fingerprint,
    DROP COLUMN IF EXISTS idempotency_key,
    DROP COLUMN IF EXISTS net_total,
    DROP COLUMN IF EXISTS discount,
    DROP COLUMN IF EXISTS subtotal,
    DROP COLUMN IF EXISTS buyer_student_alumni_year,
    DROP COLUMN IF EXISTS buyer_age,
    DROP COLUMN IF EXISTS buyer_gender,
    DROP COLUMN IF EXISTS staff_email,
    DROP COLUMN IF EXISTS staff_full_name,
    DROP COLUMN IF EXISTS staff_user_id,
    DROP COLUMN IF EXISTS status,
    DROP COLUMN IF EXISTS order_number;

DROP TABLE IF EXISTS payment_slips;
DROP TYPE IF EXISTS pos_buyer_gender;
DROP TYPE IF EXISTS pos_payment_method;
DROP TYPE IF EXISTS pos_order_status;
