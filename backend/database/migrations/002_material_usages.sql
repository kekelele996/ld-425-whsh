-- 002_material_usages.sql
-- 节点用料登记表：把材料清单与施工节点关联。
-- GORM AutoMigrate 会在应用启动时创建/更新表结构；
-- 本文件记录初始表结构，便于离线审阅与迁移追踪。
--
-- 生命周期：
--   Registered（节点完工前登记本次用料）
--     → 验收通过 Confirmed（计入材料已安装量，参与余量计算）
--     → 一次验收不通过 PendingConfirm（待确认，不参与余量计算）
--
-- 唯一约束：
--   uk_usage_node_material  同一节点同一材料只保留一条登记；
--   idx_usage_client_key    同一记录重复提交（client_key）保留第一次结果。

CREATE TABLE IF NOT EXISTS material_usages (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    project_id BIGINT UNSIGNED NOT NULL,
    node_id BIGINT UNSIGNED NOT NULL,
    material_id BIGINT UNSIGNED NOT NULL,
    quantity DECIMAL(12,2) NOT NULL DEFAULT 0,
    status VARCHAR(32) NOT NULL,
    client_key VARCHAR(64) NULL,
    note VARCHAR(200) NULL,
    created_at DATETIME(3) NULL,
    updated_at DATETIME(3) NULL,
    KEY idx_usage_project (project_id),
    KEY idx_usage_node (node_id),
    KEY idx_usage_status (status),
    UNIQUE KEY uk_usage_node_material (node_id, material_id),
    UNIQUE KEY idx_usage_client_key (client_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
