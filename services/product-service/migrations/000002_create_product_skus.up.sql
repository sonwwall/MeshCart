CREATE TABLE IF NOT EXISTS `product_skus` (
  `id` BIGINT NOT NULL,
  `spu_id` BIGINT NOT NULL,
  `sku_code` VARCHAR(64) NOT NULL,
  `title` VARCHAR(255) NOT NULL,
  `sale_price` BIGINT NOT NULL,
  `market_price` BIGINT NOT NULL DEFAULT 0,
  `status` TINYINT NOT NULL DEFAULT 0,
  `cover_url` VARCHAR(512) NOT NULL DEFAULT '',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_sku_code` (`sku_code`),
  KEY `idx_spu_id` (`spu_id`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
