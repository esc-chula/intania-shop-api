-- +goose Up
-- Manual adjustments store only an explicit operator name. POS history reads
-- its operator directly from the existing order-item link instead of copying it.
ALTER TABLE stock_transactions ADD COLUMN operator_name VARCHAR(100);

-- +goose Down
ALTER TABLE stock_transactions DROP COLUMN operator_name;
