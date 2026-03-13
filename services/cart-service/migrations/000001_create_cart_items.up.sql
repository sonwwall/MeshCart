CREATE TABLE IF NOT EXISTS `cart_items` (
  `id` BIGINT NOT NULL,
  `user_id` BIGINT NOT NULL,
  `product_id` BIGINT NOT NULL DEFAULT 0,
  `sku_id` BIGINT NOT NULL,
  `quantity` INT NOT NULL DEFAULT 1,
  `checked` TINYINT(1) NOT NULL DEFAULT 1,
  `title_snapshot` VARCHAR(255) NOT NULL DEFAULT '',
  `sku_title_snapshot` VARCHAR(255) NOT NULL DEFAULT '',
  `sale_price_snapshot` BIGINT NOT NULL DEFAULT 0,
  `cover_url_snapshot` VARCHAR(512) NOT NULL DEFAULT '',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_sku` (`user_id`, `sku_id`),
  KEY `idx_user_id` (`user_id`),
  KEY `idx_updated_at` (`updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
