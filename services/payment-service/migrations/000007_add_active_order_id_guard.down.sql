ALTER TABLE `payments`
  DROP KEY `uk_payments_active_order_id`,
  DROP COLUMN `active_order_id`;
