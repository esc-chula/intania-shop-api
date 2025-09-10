-- Revert products table changes
ALTER TABLE products 
  DROP COLUMN updated_at,
  DROP COLUMN created_at,
  DROP COLUMN stock_quantity,
  DROP COLUMN category;

ALTER TABLE products 
  RENAME COLUMN price TO base_price;

ALTER TABLE products 
  RENAME COLUMN id TO product_id;
