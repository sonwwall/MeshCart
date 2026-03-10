ALTER TABLE `products`
  DROP KEY `idx_creator_id`,
  DROP COLUMN `updated_by`,
  DROP COLUMN `creator_id`;
