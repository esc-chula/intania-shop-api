-- +goose Up
-- Minimal order record so a project can report its order count. POS checkout
-- extends this table when project orders are implemented.
CREATE TABLE orders (
    order_id BIGSERIAL PRIMARY KEY,
    project_id BIGINT REFERENCES projects(project_id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_orders_project_id ON orders(project_id);

-- +goose Down
DROP INDEX IF EXISTS idx_orders_project_id;
DROP TABLE IF EXISTS orders;
