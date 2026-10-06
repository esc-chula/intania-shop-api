-- +goose Up
-- Existing promotions had implicit AND semantics. Put every existing item in
-- its own group so their meaning is unchanged after OR groups are introduced.
ALTER TABLE promotion_items ADD COLUMN group_index INTEGER;

WITH numbered AS (
    SELECT promotion_id, project_product_id,
           ROW_NUMBER() OVER (PARTITION BY promotion_id ORDER BY project_product_id) - 1 AS group_index
    FROM promotion_items
)
UPDATE promotion_items AS pi
SET group_index = numbered.group_index
FROM numbered
WHERE pi.promotion_id = numbered.promotion_id
  AND pi.project_product_id = numbered.project_product_id;

ALTER TABLE promotion_items
    ALTER COLUMN group_index SET NOT NULL,
    ADD CONSTRAINT chk_promotion_item_group_index_nonnegative CHECK (group_index >= 0);

CREATE INDEX idx_promotion_items_promotion_group
    ON promotion_items (promotion_id, group_index, project_product_id);

-- +goose Down
DROP INDEX IF EXISTS idx_promotion_items_promotion_group;
ALTER TABLE promotion_items
    DROP CONSTRAINT IF EXISTS chk_promotion_item_group_index_nonnegative,
    DROP COLUMN IF EXISTS group_index;
