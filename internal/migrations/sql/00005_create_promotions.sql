-- +goose Up
CREATE TABLE promotions (
    promotion_id BIGSERIAL PRIMARY KEY,
    project_id BIGINT NOT NULL REFERENCES projects(project_id) ON DELETE CASCADE,
    name VARCHAR(150) NOT NULL,
    promotion_price NUMERIC(18, 2) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_promotion_name_not_empty CHECK (char_length(btrim(name)) > 0),
    CONSTRAINT chk_promotion_price_nonnegative CHECK (promotion_price >= 0),
    CONSTRAINT uq_promotions_id_project UNIQUE (promotion_id, project_id)
);

CREATE INDEX idx_promotions_project_id ON promotions (project_id, promotion_id);

CREATE TABLE promotion_items (
    promotion_id BIGINT NOT NULL,
    project_id BIGINT NOT NULL,
    project_product_id BIGINT NOT NULL,
    required_quantity INTEGER NOT NULL,
    CONSTRAINT pk_promotion_items PRIMARY KEY (promotion_id, project_product_id),
    CONSTRAINT fk_promotion_items_promotion
        FOREIGN KEY (promotion_id, project_id)
        REFERENCES promotions(promotion_id, project_id)
        ON DELETE CASCADE,
    CONSTRAINT fk_promotion_items_project_product
        FOREIGN KEY (project_product_id, project_id)
        REFERENCES project_products(project_product_id, project_id)
        ON DELETE RESTRICT,
    CONSTRAINT chk_promotion_item_quantity_positive CHECK (required_quantity > 0)
);

CREATE INDEX idx_promotion_items_project_product
    ON promotion_items(project_id, project_product_id);

-- +goose Down
DROP INDEX IF EXISTS idx_promotion_items_project_product;
DROP TABLE IF EXISTS promotion_items;
DROP INDEX IF EXISTS idx_promotions_project_id;
DROP TABLE IF EXISTS promotions;
