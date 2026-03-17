CREATE TABLE IF NOT EXISTS inventory_reservations (
    id BIGINT PRIMARY KEY,
    biz_type VARCHAR(64) NOT NULL,
    biz_id VARCHAR(128) NOT NULL,
    sku_id BIGINT NOT NULL,
    quantity BIGINT NOT NULL DEFAULT 0,
    status VARCHAR(32) NOT NULL,
    payload_snapshot TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uk_inventory_reservation_biz_sku (biz_type, biz_id, sku_id),
    KEY idx_inventory_reservation_sku_id (sku_id)
);
