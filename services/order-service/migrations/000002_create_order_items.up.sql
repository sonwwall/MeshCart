CREATE TABLE IF NOT EXISTS `order_items` (
  `id` BIGINT NOT NULL,
  `order_id` BIGINT NOT NULL,
  `product_id` BIGINT NOT NULL DEFAULT 0,
  `sku_id` BIGINT NOT NULL,
  `product_title_snapshot` VARCHAR(255) NOT NULL DEFAULT '',
  `sku_title_snapshot` VARCHAR(255) NOT NULL DEFAULT '',
  `sale_price_snapshot` BIGINT NOT NULL DEFAULT 0,
  `quantity` INT NOT NULL DEFAULT 1,
  `subtotal_amount` BIGINT NOT NULL DEFAULT 0,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_order_items_order_id` (`order_id`),
  KEY `idx_order_items_sku_id` (`sku_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
