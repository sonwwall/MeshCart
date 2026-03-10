ALTER TABLE `products`
  ADD COLUMN `creator_id` BIGINT NOT NULL DEFAULT 0 AFTER `status`,
  ADD COLUMN `updated_by` BIGINT NOT NULL DEFAULT 0 AFTER `creator_id`,
  ADD KEY `idx_creator_id` (`creator_id`);
