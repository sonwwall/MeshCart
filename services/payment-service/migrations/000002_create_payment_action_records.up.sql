CREATE TABLE IF NOT EXISTS `payment_action_records` (
  `id` BIGINT NOT NULL,
  `action_type` VARCHAR(32) NOT NULL,
  `action_key` VARCHAR(128) NOT NULL,
  `payment_id` BIGINT NOT NULL DEFAULT 0,
  `order_id` BIGINT NOT NULL DEFAULT 0,
  `status` VARCHAR(16) NOT NULL DEFAULT 'pending',
  `error_message` VARCHAR(255) NOT NULL DEFAULT '',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_payment_action_type_key` (`action_type`, `action_key`),
  KEY `idx_payment_action_payment_id` (`payment_id`),
  KEY `idx_payment_action_order_id` (`order_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
