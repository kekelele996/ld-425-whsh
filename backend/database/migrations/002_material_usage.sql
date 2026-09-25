-- 002_material_usage.sql
-- GORM AutoMigrate 会在应用启动时创建/更新表结构；
-- 本文件记录节点用料表（MaterialUsage）结构，便于离线审阅与迁移追踪。
-- 状态：Pending（已登记待计入）/ Counted（验收通过，计入已安装量）/ Excluded（验收不通过，待确认不参与余量计算）。

CREATE TABLE IF NOT EXISTS material_usages (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    project_id BIGINT UNSIGNED NOT NULL,
    node_id BIGINT UNSIGNED NOT NULL,
    material_id BIGINT UNSIGNED NOT NULL,
    quantity DECIMAL(12,2) NOT NULL DEFAULT 0,
    status VARCHAR(32) NOT NULL,
    remark VARCHAR(255),
    created_at DATETIME(3) NULL,
    updated_at DATETIME(3) NULL,
    UNIQUE KEY uk_node_material (node_id, material_id),
    KEY idx_material_usages_project (project_id),
    KEY idx_material_usages_node (node_id),
    KEY idx_material_usages_material (material_id),
    KEY idx_material_usages_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
