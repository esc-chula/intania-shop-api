-- +goose Up
-- Only an explicitly supplied name is stored. Readers fall back to the
-- staff snapshot already on the order, including for pre-existing orders.
ALTER TABLE orders ADD COLUMN operator_name VARCHAR(100);

-- +goose Down
ALTER TABLE orders DROP COLUMN operator_name;
