-- +goose Up
ALTER TABLE projects
    ADD COLUMN status_override TEXT,
    ADD CONSTRAINT chk_project_status_override
        CHECK (status_override IN ('NOT_STARTED', 'ACTIVE', 'COMPLETED'));

-- +goose Down
-- Avoid silently replacing manual choices with automatic status on rollback.
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM projects WHERE status_override IS NOT NULL) THEN
        RAISE EXCEPTION 'cannot roll back project status override while overrides exist';
    END IF;
END $$;
-- +goose StatementEnd

ALTER TABLE projects DROP CONSTRAINT chk_project_status_override;
ALTER TABLE projects DROP COLUMN status_override;
