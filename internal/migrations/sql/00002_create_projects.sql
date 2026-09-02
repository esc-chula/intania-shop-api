-- +goose Up
CREATE TABLE projects (
    project_id BIGSERIAL PRIMARY KEY,
    name VARCHAR(150) NOT NULL,
    description TEXT,
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_project_dates CHECK (start_date <= end_date)
);

CREATE INDEX idx_projects_listing ON projects(created_at DESC, project_id DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_projects_listing;
DROP TABLE IF EXISTS projects;
