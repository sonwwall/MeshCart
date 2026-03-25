UPDATE `payments`
SET `active_order_id` = NULL
WHERE `status` = 1;
