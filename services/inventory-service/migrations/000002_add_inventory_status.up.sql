ALTER TABLE `inventory_stocks`
  ADD COLUMN `status` TINYINT NOT NULL DEFAULT 1 AFTER `available_stock`,
  ADD KEY `idx_status` (`status`);
