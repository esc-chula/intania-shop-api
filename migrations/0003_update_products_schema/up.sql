-- Update products table to match our implementation
ALTER TABLE products 
  RENAME COLUMN product_id TO id;

ALTER TABLE products 
  RENAME COLUMN base_price TO price;

ALTER TABLE products 
  ADD COLUMN category VARCHAR(100),
  ADD COLUMN stock_quantity INTEGER,
  ADD COLUMN created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  ADD COLUMN updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP;
