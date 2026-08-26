-- +goose Up
CREATE TABLE projects (
    project_id BIGSERIAL PRIMARY KEY,
    name VARCHAR(150) NOT NULL,
    description TEXT,
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_project_dates CHECK (start_date <= end_date)
);

ALTER TABLE orders
  ADD COLUMN project_id BIGINT
    REFERENCES projects(project_id)
    ON DELETE RESTRICT;

CREATE INDEX idx_orders_project_id ON orders(project_id);

-- +goose Down
DROP INDEX idx_orders_project_id;

ALTER TABLE orders DROP COLUMN project_id;

DROP TABLE projects;
