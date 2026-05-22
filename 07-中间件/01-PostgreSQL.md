# PostgreSQL 选型与用法

**版本要求：** ≥ 15.x  
**角色：** 关系型主库（所有需要强一致事务的业务数据）

---

## 一、选型理由

| 特性 | 说明 |
|------|------|
| JSONB | 存储模型配置、路由策略等半结构化数据，支持 GIN 索引 |
| 行级安全（RLS） | 租户数据隔离，SQL 层原生支持，无需应用层 WHERE |
| 声明式分区 | billing_ledger、audit_log 等大表按月 RANGE 分区，查询剪枝 |
| 逻辑复制 | 支持只读副本横向扩展读流量 |
| pg_partman | 自动创建/删除分区，免人工 DDL |

---

## 二、数据库分布规划

| 数据库名 | 归属服务 | 说明 |
|---------|---------|------|
| `maas_auth` | auth-service | 用户、角色、权限、SSO |
| `maas_routing` | routing-service | 路由策略、Fallback 链 |
| `maas_catalog` | model-catalog-service | 供应商、模型元数据 |
| `maas_billing` | billing-service | billing_ledger、预算配置 |
| `maas_compliance` | compliance-service | 审计日志、数据分级规则 |
| `maas_prompt` | prompt-eval-service | Prompt 模板、评测任务 |
| `maas_notification` | notification-service | 告警规则、通知配置 |

> 每个数据库使用独立 PostgreSQL 用户，最小权限原则（仅 CONNECT + 业务表 CRUD）。

---

## 三、连接池配置（PgBouncer）

```ini
[databases]
maas_auth     = host=pg-primary port=5432 dbname=maas_auth
maas_billing  = host=pg-primary port=5432 dbname=maas_billing

[pgbouncer]
pool_mode         = transaction        # 事务级连接池
max_client_conn   = 2000
default_pool_size = 20
min_pool_size     = 5
reserve_pool_size = 5
server_idle_timeout = 600
log_connections   = 0
log_disconnections= 0
```

**Go 侧连接串示例（使用 pgx）：**

```go
config, _ := pgxpool.ParseConfig(
    "postgres://maas_auth_user:xxx@pgbouncer:5432/maas_auth" +
    "?pool_max_conns=20&pool_min_conns=2&pool_max_conn_lifetime=30m",
)
pool, _ := pgxpool.NewWithConfig(ctx, config)
```

---

## 四、分区表规范

```sql
-- billing_ledger 按月分区（示例）
CREATE TABLE billing_ledger (
    id          BIGSERIAL,
    tenant_id   UUID        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    ...
) PARTITION BY RANGE (created_at);

-- 由 pg_partman 自动维护月分区
SELECT partman.create_parent(
    p_parent_table => 'public.billing_ledger',
    p_control      => 'created_at',
    p_type         => 'range',
    p_interval     => 'monthly'
);
```

---

## 五、行级安全（RLS）

```sql
-- 在 maas_billing 开启 RLS
ALTER TABLE billing_ledger ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON billing_ledger
    USING (tenant_id = current_setting('app.current_tenant_id')::uuid);

-- 应用层在事务开始时注入上下文
SET LOCAL app.current_tenant_id = '{{tenant_uuid}}';
```

---

## 六、备份策略

| 类型 | 工具 | 频率 | 保留 |
|------|------|------|------|
| 物理备份 | pgBackRest | 每日全量 | 30天 |
| WAL 归档 | pgBackRest WAL Archive | 实时 | 7天 |
| 逻辑快照 | pg_dump | 每周 | 4周 |

---

## 七、慢查询监控

```sql
-- 开启慢查询日志（postgresql.conf）
log_min_duration_statement = 200   -- 超 200ms 记录
log_line_prefix = '%t [%p] %u@%d '

-- 查询 Top 慢 SQL
SELECT query, calls, mean_exec_time
FROM pg_stat_statements
ORDER BY mean_exec_time DESC
LIMIT 20;
```
