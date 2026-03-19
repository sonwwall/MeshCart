ALTER TABLE `payments`
  MODIFY COLUMN `expire_at` DATETIME NOT NULL,
  ADD KEY `idx_payments_status_expire_at` (`status`, `expire_at`);
