ALTER TABLE `users`
ADD COLUMN `role` VARCHAR(32) NOT NULL DEFAULT 'user' AFTER `password`,
ADD KEY `idx_role` (`role`);

UPDATE `users`
SET `role` = 'superadmin'
WHERE `id` = (
    SELECT `id` FROM (
        SELECT `id`
        FROM `users`
        ORDER BY `created_at` ASC, `id` ASC
        LIMIT 1
    ) AS `bootstrap_user`
);
