UPDATE `payments`
SET `expire_at` = COALESCE(
  `closed_at`,
  `succeeded_at`,
  DATE_ADD(`created_at`, INTERVAL 15 MINUTE)
)
WHERE `expire_at` IS NULL;
