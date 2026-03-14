ALTER TABLE `product_skus`
  DROP INDEX `idx_sku_code`,
  MODIFY COLUMN `sku_code` VARCHAR(64) NOT NULL,
  ADD UNIQUE KEY `uk_sku_code` (`sku_code`);
