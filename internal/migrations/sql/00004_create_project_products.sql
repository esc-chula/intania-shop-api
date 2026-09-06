-- +goose Up
CREATE TABLE project_products (
    project_product_id BIGSERIAL PRIMARY KEY,
    project_id BIGINT NOT NULL REFERENCES projects(project_id) ON DELETE CASCADE,
    product_id BIGINT NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
    variant_id BIGINT REFERENCES variants(variant_id) ON DELETE RESTRICT,
    project_price NUMERIC(10, 2) NOT NULL CHECK (project_price >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- PostgreSQL treats NULLs as distinct in a normal unique constraint. This
-- expression index makes the variantless sellable item unique as well.
CREATE UNIQUE INDEX uq_project_products_identity
    ON project_products (project_id, product_id, COALESCE(variant_id, 0));
CREATE UNIQUE INDEX uq_project_products_id_project
    ON project_products (project_product_id, project_id);
CREATE INDEX idx_project_products_project ON project_products (project_id);

-- These tables provide the assignment references required to prevent a
-- promotion from silently changing meaning when the selection is replaced.
CREATE TABLE project_promotions (
    promotion_id BIGSERIAL PRIMARY KEY,
    project_id BIGINT NOT NULL REFERENCES projects(project_id) ON DELETE CASCADE,
    name VARCHAR(150) NOT NULL,
    promotion_price NUMERIC(10, 2) NOT NULL CHECK (promotion_price >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX uq_project_promotions_id_project
    ON project_promotions (promotion_id, project_id);

CREATE TABLE project_promotion_items (
    promotion_id BIGINT NOT NULL REFERENCES project_promotions(promotion_id) ON DELETE CASCADE,
    project_id BIGINT NOT NULL REFERENCES projects(project_id) ON DELETE CASCADE,
    project_product_id BIGINT NOT NULL REFERENCES project_products(project_product_id) ON DELETE RESTRICT,
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    PRIMARY KEY (promotion_id, project_product_id),
    FOREIGN KEY (promotion_id, project_id)
        REFERENCES project_promotions(promotion_id, project_id) ON DELETE CASCADE,
    FOREIGN KEY (project_product_id, project_id)
        REFERENCES project_products(project_product_id, project_id) ON DELETE RESTRICT
);

-- +goose Down
DROP TABLE IF EXISTS project_promotion_items;
DROP TABLE IF EXISTS project_promotions;
DROP TABLE IF EXISTS project_products;
