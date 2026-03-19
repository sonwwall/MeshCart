ALTER TABLE `payments`
  ADD COLUMN `expire_at` DATETIME NOT NULL AFTER `request_id`,
  ADD KEY `idx_payments_status_expire_at` (`status`, `expire_at`);
