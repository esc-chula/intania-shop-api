-- +goose Up
ALTER TABLE order_promotions
    ADD COLUMN application_count BIGINT
    CHECK (application_count > 0);

-- +goose Down
ALTER TABLE order_promotions DROP COLUMN application_count;
