ALTER TABLE `payments`
  ADD COLUMN `active_order_id` BIGINT NULL AFTER `order_id`;

UPDATE `payments`
SET `active_order_id` = `order_id`
WHERE `status` = 1;

ALTER TABLE `payments`
  ADD UNIQUE KEY `uk_payments_active_order_id` (`active_order_id`);
