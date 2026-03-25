ALTER TABLE `payments`
  ADD UNIQUE KEY `uk_payments_active_order_id` (`active_order_id`);
