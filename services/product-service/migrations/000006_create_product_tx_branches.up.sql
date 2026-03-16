CREATE TABLE IF NOT EXISTS product_tx_branches (
    id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    global_tx_id VARCHAR(128) NOT NULL,
    branch_id VARCHAR(128) NOT NULL,
    action VARCHAR(64) NOT NULL,
    biz_id BIGINT NOT NULL DEFAULT 0,
    status VARCHAR(32) NOT NULL,
    payload_snapshot TEXT NOT NULL,
    error_message VARCHAR(255) NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uk_product_tx_branch_action (global_tx_id, branch_id, action),
    KEY idx_product_tx_branch_biz_id (biz_id)
);
