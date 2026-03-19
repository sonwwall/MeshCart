ALTER TABLE `payments`
  DROP KEY `idx_payments_status_expire_at`,
  MODIFY COLUMN `expire_at` DATETIME NULL;
