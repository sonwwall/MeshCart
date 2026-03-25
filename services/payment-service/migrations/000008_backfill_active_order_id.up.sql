UPDATE `payments`
SET `active_order_id` = `order_id`
WHERE `status` = 1;
