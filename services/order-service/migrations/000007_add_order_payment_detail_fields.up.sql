ALTER TABLE `orders`
  ADD COLUMN `payment_method` VARCHAR(32) NOT NULL DEFAULT '' AFTER `payment_id`,
  ADD COLUMN `payment_trade_no` VARCHAR(128) NOT NULL DEFAULT '' AFTER `payment_method`;
