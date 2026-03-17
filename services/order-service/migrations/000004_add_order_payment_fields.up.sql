ALTER TABLE `orders`
  ADD COLUMN `payment_id` VARCHAR(128) NOT NULL DEFAULT '' AFTER `cancel_reason`,
  ADD COLUMN `paid_at` DATETIME NULL AFTER `payment_id`;
