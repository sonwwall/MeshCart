ALTER TABLE `payments`
  DROP KEY `idx_payments_status_expire_at`,
  DROP COLUMN `expire_at`;
