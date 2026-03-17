CREATE TABLE IF NOT EXISTS `orders` (
  `order_id` BIGINT NOT NULL,
  `user_id` BIGINT NOT NULL,
  `status` TINYINT NOT NULL DEFAULT 1,
  `total_amount` BIGINT NOT NULL DEFAULT 0,
  `pay_amount` BIGINT NOT NULL DEFAULT 0,
  `expire_at` DATETIME NOT NULL,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`order_id`),
  KEY `idx_orders_user_id_status` (`user_id`, `status`),
  KEY `idx_orders_updated_at` (`updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
