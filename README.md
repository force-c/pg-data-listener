# PostgreSQL 通用数据变更监听

基于 PostgreSQL LISTEN/NOTIFY 实现通用的多表实时监听和热更新。

## 特性

✅ **通用触发器** - 一个触发器函数适配所有表  
✅ **自动序列化** - 使用 `row_to_json()` 自动序列化任意字段  
✅ **统一 Channel** - 单一 `data_changes` 频道，易于管理  
✅ **灵活路由** - Go 端基于表名自动路由到对应 Handler  
✅ **扩展简单** - 新增表只需：1) 添加触发器 2) 实现 Handler  

## 架构设计

### PostgreSQL 端
```sql
-- 1️⃣ 通用触发器函数（适配所有表）
CREATE FUNCTION generic_table_notify() ...

-- 2️⃣ 为每个表绑定触发器
CREATE TRIGGER xxx_trigger ... EXECUTE FUNCTION generic_table_notify();
```

**Payload 格式：**
```json
{
  "table": "s_config",
  "operation": "UPDATE",
  "data": {"id": 1, "config_key": "app_name", ...},
  "timestamp": "2024-01-01T12:00:00Z"
}
```

### Go 端
```go
// 1️⃣ 定义 Handler 接口
type TableChangeHandler interface {
    HandleChange(operation string, data json.RawMessage) error
}

// 2️⃣ 每个表实现自己的 Manager（实现 Handler 接口）
type ConfigManager struct { ... }
func (cm *ConfigManager) HandleChange(...) { ... }

// 3️⃣ 注册并启动监听
listener.RegisterHandler("s_config", configManager)
listener.RegisterHandler("s_user", userManager)
listener.Start(connStr)
```

## 使用步骤

### 1. 初始化数据库
```bash
psql -U postgres -d testdb -f schema.sql
```

### 2. 修改连接字符串
编辑 `main.go:280`：
```go
connStr := "host=localhost port=5432 user=postgres password=yourpass dbname=testdb sslmode=disable"
```

### 3. 运行程序
```bash
go run main.go
```

### 4. 测试变更
```sql
-- 测试 s_config
UPDATE s_config SET config_value = 'true' WHERE config_key = 'debug_mode';
INSERT INTO s_config (config_key, config_value, description) VALUES ('new_key', 'value', 'test');
DELETE FROM s_config WHERE config_key = 'new_key';

-- 测试 s_user
UPDATE s_user SET status = 'inactive' WHERE username = 'test_user';
INSERT INTO s_user (username, email, status) VALUES ('new_user', 'new@example.com', 'active');
DELETE FROM s_user WHERE username = 'new_user';
```

## 扩展新表

### 1. 在 schema.sql 中添加表和触发器
```sql
CREATE TABLE s_product (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100),
    price DECIMAL(10,2)
);

CREATE TRIGGER s_product_trigger
AFTER INSERT OR UPDATE OR DELETE ON s_product
FOR EACH ROW EXECUTE FUNCTION generic_table_notify();
```

### 2. 在 main.go 中实现 Handler
```go
type Product struct {
    ID    int     `json:"id"`
    Name  string  `json:"name"`
    Price float64 `json:"price"`
}

type ProductManager struct {
    products map[int]Product
    mu       sync.RWMutex
}

func (pm *ProductManager) HandleChange(operation string, data json.RawMessage) error {
    var product Product
    json.Unmarshal(data, &product)
    
    pm.mu.Lock()
    defer pm.mu.Unlock()
    
    switch operation {
    case "INSERT", "UPDATE":
        pm.products[product.ID] = product
    case "DELETE":
        delete(pm.products, product.ID)
    }
    return nil
}
```

### 3. 注册 Handler
```go
productManager := NewProductManager(listener.db)
listener.RegisterHandler("s_product", productManager)
```

## 优势对比

| 方案 | 触发器数量 | Channel 数量 | 扩展复杂度 | 代码量 |
|------|-----------|-------------|-----------|--------|
| 传统方案 | N个(每表不同) | N个 | 高 | 大 |
| **本方案** | **1个(通用)** | **1个** | **低** | **小** |

## 项目结构

```
.
├── schema.sql          # 数据库表结构 + 通用触发器
├── main.go            # Go 监听程序
│   ├── DataListener      # 统一监听器（LISTEN/NOTIFY）
│   ├── ConfigManager     # s_config 表的 Handler
│   └── UserManager       # s_user 表的 Handler
├── go.mod
└── README.md
```

## 技术要点

- **`row_to_json()`**: 自动将表行转为 JSON，无需手动列举字段
- **`TG_TABLE_NAME`**: 触发器变量，自动获取当前表名
- **`json.RawMessage`**: Go 端延迟解析，支持不同表结构
- **接口设计**: `TableChangeHandler` 接口实现松耦合
