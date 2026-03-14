ALTER TABLE `product_skus`
  DROP INDEX `uk_sku_code`,
  MODIFY COLUMN `sku_code` VARCHAR(64) NOT NULL DEFAULT '',
  ADD KEY `idx_sku_code` (`sku_code`);
