CREATE TABLE IF NOT EXISTS `payment_outbox` (
  `id` BIGINT NOT NULL,
  `topic` VARCHAR(128) NOT NULL,
  `event_name` VARCHAR(128) NOT NULL,
  `event_key` VARCHAR(128) NOT NULL DEFAULT '',
  `producer` VARCHAR(64) NOT NULL,
  `headers_json` JSON NOT NULL,
  `body` JSON NOT NULL,
  `status` VARCHAR(16) NOT NULL DEFAULT 'pending',
  `retry_count` INT NOT NULL DEFAULT 0,
  `max_retries` INT NOT NULL DEFAULT 16,
  `next_retry_at` DATETIME NULL,
  `last_error` VARCHAR(255) NOT NULL DEFAULT '',
  `published_at` DATETIME NULL,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_payment_outbox_status_retry` (`status`, `next_retry_at`),
  KEY `idx_payment_outbox_event_key` (`event_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

