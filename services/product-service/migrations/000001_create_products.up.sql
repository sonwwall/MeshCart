CREATE TABLE IF NOT EXISTS `products` (
  `id` BIGINT NOT NULL,
  `title` VARCHAR(255) NOT NULL,
  `sub_title` VARCHAR(255) NOT NULL DEFAULT '',
  `category_id` BIGINT NOT NULL DEFAULT 0,
  `brand` VARCHAR(128) NOT NULL DEFAULT '',
  `description` TEXT NULL,
  `status` TINYINT NOT NULL DEFAULT 0,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_category_id` (`category_id`),
  KEY `idx_status` (`status`),
  KEY `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
