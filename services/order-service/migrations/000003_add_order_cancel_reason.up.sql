ALTER TABLE `orders`
  ADD COLUMN `cancel_reason` VARCHAR(255) NOT NULL DEFAULT '' AFTER `expire_at`;
