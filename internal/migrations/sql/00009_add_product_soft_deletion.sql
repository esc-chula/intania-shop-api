-- +goose Up
-- Archived products remain referenced by paid orders, stock history, and
-- completed project configuration. NULL means the product is still active.
ALTER TABLE products
    ADD COLUMN deleted_at TIMESTAMPTZ;

CREATE INDEX idx_products_active_id
    ON products (id DESC)
    WHERE deleted_at IS NULL;

-- Product archival checks every project assignment for the product. The
-- existing unique index starts with project_id, so it cannot efficiently
-- support that lookup.
CREATE INDEX idx_project_products_product
    ON project_products (product_id, project_id);

-- +goose Down
-- Dropping this column after it has been used would silently reactivate
-- archived products. Restore/resolve those rows explicitly before rollback.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM products WHERE deleted_at IS NOT NULL) THEN
        RAISE EXCEPTION 'cannot roll back product soft deletion while archived products exist';
    END IF;
END $$;

DROP INDEX IF EXISTS idx_project_products_product;
DROP INDEX IF EXISTS idx_products_active_id;
ALTER TABLE products DROP COLUMN IF EXISTS deleted_at;
