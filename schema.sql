-- ===========================
-- 通用触发器函数（适用所有表）
-- ===========================
CREATE OR REPLACE FUNCTION generic_table_notify()
RETURNS TRIGGER AS $$
DECLARE
    payload JSON;
    row_data JSON;
BEGIN
    -- 根据操作类型选择 OLD 或 NEW
    IF TG_OP = 'DELETE' THEN
        row_data = row_to_json(OLD);
    ELSE
        row_data = row_to_json(NEW);
    END IF;
    
    -- 构建通知 payload
    payload = json_build_object(
        'table', TG_TABLE_NAME,
        'operation', TG_OP,
        'data', row_data,
        'timestamp', CURRENT_TIMESTAMP
    );
    
    -- 发送到统一 channel
    PERFORM pg_notify('data_changes', payload::text);
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

-- ===========================
-- 配置表
-- ===========================
CREATE TABLE IF NOT EXISTS s_config (
    id SERIAL PRIMARY KEY,
    config_key VARCHAR(100) NOT NULL UNIQUE,
    config_value TEXT NOT NULL,
    description VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_s_config_key ON s_config(config_key);

-- 绑定通用触发器
DROP TRIGGER IF EXISTS s_config_change_trigger ON s_config;
CREATE TRIGGER s_config_change_trigger
AFTER INSERT OR UPDATE OR DELETE ON s_config
FOR EACH ROW EXECUTE FUNCTION generic_table_notify();

-- ===========================
-- 用户表（示例）
-- ===========================
CREATE TABLE IF NOT EXISTS s_user (
    id SERIAL PRIMARY KEY,
    username VARCHAR(50) NOT NULL UNIQUE,
    email VARCHAR(100) NOT NULL,
    status VARCHAR(20) DEFAULT 'active',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_s_user_username ON s_user(username);

-- 绑定通用触发器
DROP TRIGGER IF EXISTS s_user_change_trigger ON s_user;
CREATE TRIGGER s_user_change_trigger
AFTER INSERT OR UPDATE OR DELETE ON s_user
FOR EACH ROW EXECUTE FUNCTION generic_table_notify();

-- ===========================
-- 示例数据
-- ===========================
INSERT INTO s_config (config_key, config_value, description) VALUES
    ('app_name', 'MyApp', '应用名称'),
    ('max_connections', '100', '最大连接数'),
    ('debug_mode', 'false', '调试模式')
ON CONFLICT (config_key) DO NOTHING;

INSERT INTO s_user (username, email, status) VALUES
    ('admin', 'admin@example.com', 'active'),
    ('test_user', 'test@example.com', 'active')
ON CONFLICT (username) DO NOTHING;
