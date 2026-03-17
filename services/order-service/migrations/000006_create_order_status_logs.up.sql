CREATE TABLE IF NOT EXISTS `order_status_logs` (
  `id` BIGINT NOT NULL,
  `order_id` BIGINT NOT NULL,
  `from_status` TINYINT NOT NULL DEFAULT 0,
  `to_status` TINYINT NOT NULL,
  `action_type` VARCHAR(32) NOT NULL DEFAULT '',
  `reason` VARCHAR(255) NOT NULL DEFAULT '',
  `external_ref` VARCHAR(128) NOT NULL DEFAULT '',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_order_status_logs_order_id` (`order_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
