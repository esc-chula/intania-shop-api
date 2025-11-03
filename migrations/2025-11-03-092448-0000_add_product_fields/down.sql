-- This file should undo anything in `up.sql`
ALTER TABLE products 
    DROP COLUMN IF EXISTS shipping,
    DROP COLUMN IF EXISTS preview_video,
    DROP COLUMN IF EXISTS preview_image;
