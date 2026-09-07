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

-- +goose Down
DROP TABLE IF EXISTS project_products;
