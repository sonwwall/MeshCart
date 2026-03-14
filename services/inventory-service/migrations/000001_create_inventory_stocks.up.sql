CREATE TABLE `inventory_stocks` (
  `id` BIGINT NOT NULL,
  `sku_id` BIGINT NOT NULL,
  `total_stock` BIGINT NOT NULL DEFAULT 0,
  `reserved_stock` BIGINT NOT NULL DEFAULT 0,
  `available_stock` BIGINT NOT NULL DEFAULT 0,
  `version` BIGINT NOT NULL DEFAULT 1,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_sku_id` (`sku_id`),
  KEY `idx_updated_at` (`updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
