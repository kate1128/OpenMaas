# ClickHouse 选型与用法

**版本要求：** ≥ 24.3 LTS  
**角色：** OLAP 分析引擎（LLMOps Trace 存储与查询）

---

## 一、选型理由

| 特性 | 说明 |
|------|------|
| 列式存储 | 44 字段 Trace 表查询只读所需列，I/O 极低 |
| LZ4/ZSTD 压缩 | Trace 数据压缩比可达 10:1，降低存储成本 |
| MergeTree 引擎 | 支持 TTL、分区剪枝、物化视图，无需 DBA 手动维护 |
| 亿级写入 | 批量写入（INSERT batch）可达数十万行/秒 |
| PromQL-like SQL | 时序聚合函数（quantile、toStartOfMinute）直接支持 |

---

## 二、核心表设计

```sql
CREATE TABLE llm_traces
(
    -- 基础标识
    trace_id        String,
    tenant_id       String,
    project_id      String,
    api_key_id      String,
    -- 请求信息
    model_id        String,
    vendor_id       String,
    route_policy_id String,
    -- Token 计量
    prompt_tokens   UInt32,
    completion_tokens UInt32,
    total_tokens    UInt32,
    cached_tokens   UInt32,
    -- 延迟
    latency_ms      UInt32,
    ttfb_ms         UInt32,
    -- 状态
    status          LowCardinality(String),   -- success/error/timeout
    error_code      String,
    -- 成本
    cost_usd        Decimal(12,6),
    -- 时间
    created_at      DateTime64(3, 'UTC'),
    date            Date MATERIALIZED toDate(created_at)
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(created_at)
ORDER BY (tenant_id, created_at, trace_id)
TTL created_at + INTERVAL 180 DAY DELETE
SETTINGS index_granularity = 8192;
```

---

## 三、物化视图（分钟级聚合）

```sql
-- 用于实时监控 Dashboard，避免全表扫描
CREATE MATERIALIZED VIEW llm_traces_minute_mv
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMMDD(minute)
ORDER BY (tenant_id, model_id, minute)
AS SELECT
    tenant_id,
    model_id,
    toStartOfMinute(created_at) AS minute,
    count()                      AS request_count,
    sum(total_tokens)            AS total_tokens,
    sum(cost_usd)                AS total_cost,
    quantileState(0.99)(latency_ms) AS p99_latency_state,
    countIf(status = 'error')   AS error_count
FROM llm_traces
GROUP BY tenant_id, model_id, minute;
```

---

## 四、Go 写入（批量 INSERT）

```go
import "github.com/ClickHouse/clickhouse-go/v2"

conn, _ := clickhouse.Open(&clickhouse.Options{
    Addr: []string{"clickhouse:9000"},
    Auth: clickhouse.Auth{Database: "maas", Username: "maas_writer"},
    Settings: clickhouse.Settings{
        "max_insert_block_size": 100000,
    },
    Compression: &clickhouse.Compression{Method: clickhouse.CompressionLZ4},
})

// 批量写入
batch, _ := conn.PrepareBatch(ctx, "INSERT INTO llm_traces")
for _, t := range traces {
    batch.Append(t.TraceID, t.TenantID, /* ... */)
}
batch.Send()
```

> 建议攒批 500~2000 条或 1s 超时后触发写入，避免频繁小批量写入。

---

## 五、TTL 分层存储

```sql
-- 热数据 0-7天保留在 SSD，暖数据 8-30天移到 HDD，30天后压缩
ALTER TABLE llm_traces MODIFY TTL
    created_at + INTERVAL 7 DAY TO DISK 'hdd',
    created_at + INTERVAL 30 DAY TO VOLUME 'cold',
    created_at + INTERVAL 180 DAY DELETE;
```

---

## 六、查询优化规范

1. **必须带分区键过滤**：`WHERE created_at >= ... AND tenant_id = ...`
2. **禁止 SELECT \***：明确列举查询字段
3. **P99 延迟查询用物化视图**，避免实时 quantile 全表扫描
4. **大范围聚合**加 `LIMIT 1000` 防止 OOM
5. 连接池复用，使用 `clickhouse-go/v2` 原生协议（端口 9000），禁止 HTTP 接口大批量写入

---

## 七、监控指标

| 指标 | 告警阈值 |
|------|---------|
| `ClickHouseAsyncInsertQueue` 队列深度 | > 50000 |
| 单查询内存 `query_memory_usage` | > 4GB |
| 磁盘使用率 | > 75% |
| Merge 积压 `NumberOfPartsToMerge` | > 300 |
