-- Development-only fixture for the local Docker database.
-- This removes all catalogue, project, inventory, promotion, payment-slip,
-- and order data. It deliberately preserves users created through Google OAuth.

BEGIN;

TRUNCATE TABLE payment_slips, projects, products RESTART IDENTITY CASCADE;

INSERT INTO products
    (name, description, price, status, category, stock_quantity, images, product_type, sku, product_code)
VALUES
    ('Sticker Set', 'Intania Music Fest 2026 sticker set.', 122.00, 'IN_STOCK', 'Accessories', 100, '{}', 'SINGLE', 'IMF-STK-2026', 'STK-2026'),
    ('Intania Polo', 'Classic Intania polo shirt. Choose your size.', 890.00, 'IN_STOCK', 'Apparel', NULL, '{}', 'MULTIPLE', 'INT-POLO-001', 'POLO-001'),
    ('Random Keychain', 'A surprise Intania Music Fest 2026 keychain.', 69.00, 'IN_STOCK', 'Accessories', 250, '{}', 'SINGLE', 'IMF-KEY-2026', 'KEY-2026'),
    ('INTANIA Cap White', 'White INTANIA cap.', 390.00, 'IN_STOCK', 'Apparel', 50, '{}', 'SINGLE', 'INT-CAP-WHT', 'CAP-WHT'),
    ('INTANIA Cap Red', 'Red INTANIA cap.', 390.00, 'IN_STOCK', 'Apparel', 50, '{}', 'SINGLE', 'INT-CAP-RED', 'CAP-RED');

INSERT INTO variants (product_id, size, color, stock_quantity, price)
VALUES
    (2, 'M', 'Navy', 20, 890.00),
    (2, 'L', 'Navy', 15, 890.00);

INSERT INTO stock_transactions
    (product_id, variant_id, transaction_type, quantity_change, quantity_before, quantity_after, reason, notes, reference_type, created_by)
VALUES
    (1, NULL, 'INITIAL', 100, 0, 100, 'Local seed', 'Development fixture', 'SEED', 'local-seed'),
    (NULL, 1, 'INITIAL', 20, 0, 20, 'Local seed', 'Development fixture', 'SEED', 'local-seed'),
    (NULL, 2, 'INITIAL', 15, 0, 15, 'Local seed', 'Development fixture', 'SEED', 'local-seed'),
    (3, NULL, 'INITIAL', 250, 0, 250, 'Local seed', 'Development fixture', 'SEED', 'local-seed'),
    (4, NULL, 'INITIAL', 50, 0, 50, 'Local seed', 'Development fixture', 'SEED', 'local-seed'),
    (5, NULL, 'INITIAL', 50, 0, 50, 'Local seed', 'Development fixture', 'SEED', 'local-seed');

INSERT INTO projects (name, description, start_date, end_date)
VALUES (
    'Intania Shop Demo Sale',
    'Intania Music Fest 2026 merchandise for local catalogue and POS screens.',
    (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Bangkok')::date - 1,
    (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Bangkok')::date + 30
);

INSERT INTO project_products (project_id, product_id, variant_id, project_price)
VALUES
    (1, 1, NULL, 122.00),
    (1, 2, 1, 890.00),
    (1, 2, 2, 890.00),
    (1, 3, NULL, 69.00),
    (1, 4, NULL, 390.00),
    (1, 5, NULL, 390.00);

INSERT INTO promotions (project_id, name, promotion_price)
VALUES (1, 'Swagged Out Set', 990.00);

INSERT INTO promotion_items (promotion_id, project_id, project_product_id, required_quantity)
VALUES
    (1, 1, 1, 1),
    (1, 1, 2, 1);

COMMIT;
