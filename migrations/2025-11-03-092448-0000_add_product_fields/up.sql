-- Your SQL goes here
ALTER TABLE products 
    ADD COLUMN IF NOT EXISTS preview_image TEXT[],
    ADD COLUMN IF NOT EXISTS preview_video TEXT[],
    ADD COLUMN IF NOT EXISTS shipping TEXT[];