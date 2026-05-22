# Redis 选型与用法

**版本要求：** ≥ 7.2（Redis Stack，含 RedisSearch 向量模块）  
**部署模式：** Redis Cluster（3 主 3 从）

---

## 一、选型理由

| 特性 | 用途 |
|------|------|
| String / Hash | Token 缓存、会话存储 |
| Sorted Set | 限流滑动窗口、排行榜 |
| Stream | 轻量事件队列（告警去重） |
| Lua 脚本 | 原子预扣余额 |
| RedisSearch（RediSearch） | 语义缓存向量检索（余弦相似度） |
| Cluster 模式 | 水平分片，单集群支持 TB 级热数据 |

---

## 二、Key 命名规范

```
{namespace}:{resource_type}:{id}[:{sub_key}]
```

| Key 模式 | TTL | 说明 |
|---------|-----|------|
| `maas:token:{jti}` | access_token 有效期 | JWT 黑名单（登出后写入） |
| `maas:session:{sid}` | 30min 滑动 | 用户会话 |
| `maas:ratelimit:{tenant_id}:{window}` | 60s | 滑动窗口限流计数 |
| `maas:budget:pre:{key_id}` | 5min | 预扣 Token 余额 |
| `maas:model:list:{tenant_id}` | 60s | 模型列表缓存 |
| `maas:route:policy:{policy_id}` | 30s | 路由策略热缓存 |
| `maas:dedup:alert:{fingerprint}` | 5min | 告警去重窗口 |
| `maas:cache:semantic:{hash}` | 1h | 语义缓存命中记录 |

---

## 三、限流实现（Lua 滑动窗口）

```lua
-- KEYS[1] = rate_limit key, ARGV[1] = window_ms, ARGV[2] = limit, ARGV[3] = now_ms
local key    = KEYS[1]
local window = tonumber(ARGV[1])
local limit  = tonumber(ARGV[2])
local now    = tonumber(ARGV[3])

redis.call('ZREMRANGEBYSCORE', key, '-inf', now - window)
local count = redis.call('ZCARD', key)
if count >= limit then
    return 0
end
redis.call('ZADD', key, now, now)
redis.call('PEXPIRE', key, window)
return 1
```

Go 调用：

```go
allowed, err := rdb.Eval(ctx, luaScript,
    []string{fmt.Sprintf("maas:ratelimit:%s:1min", tenantID)},
    60000, 1000, time.Now().UnixMilli(),
).Int()
```

---

## 四、预扣余额（原子 Lua）

```lua
-- KEYS[1] = budget key, ARGV[1] = amount
local balance = tonumber(redis.call('GET', KEYS[1]) or 0)
if balance < tonumber(ARGV[1]) then return -1 end
return redis.call('DECRBY', KEYS[1], ARGV[1])
```

---

## 五、语义缓存（RedisSearch 向量检索）

```bash
# 创建向量索引
FT.CREATE maas_semantic_cache
  ON HASH PREFIX 1 maas:sc:
  SCHEMA
    prompt_hash TEXT
    embedding   VECTOR HNSW 6 TYPE FLOAT32 DIM 1024 DISTANCE_METRIC COSINE
    response    TEXT
    created_at  NUMERIC

# 检索最近邻（余弦相似度 ≥ 0.92）
FT.SEARCH maas_semantic_cache
  "*=>[KNN 1 @embedding $vec AS score]"
  PARAMS 2 vec {query_embedding_bytes}
  FILTER @score >= 0.08   -- 余弦距离 ≤ 0.08 即相似度 ≥ 0.92
  RETURN 2 response score
```

Go 侧使用 `go-redis/v9` + `rueidis` 调用 FT.SEARCH。

---

## 六、Cluster 配置要点

```yaml
# redis.conf 关键配置
cluster-enabled yes
cluster-node-timeout 5000
cluster-migration-barrier 1
maxmemory 8gb
maxmemory-policy allkeys-lru       # 非持久化 Key 使用 LRU
appendonly yes                      # AOF 持久化
appendfsync everysec
```

**Go 客户端（go-redis Cluster）：**

```go
rdb := redis.NewClusterClient(&redis.ClusterOptions{
    Addrs:        []string{"redis-0:6379", "redis-1:6379", "redis-2:6379"},
    Password:     os.Getenv("REDIS_PASSWORD"),
    PoolSize:     20,
    MinIdleConns: 5,
    ReadTimeout:  200 * time.Millisecond,
    WriteTimeout: 200 * time.Millisecond,
})
```

---

## 七、监控指标（Prometheus redis_exporter）

| 指标 | 告警阈值 |
|------|---------|
| `redis_memory_used_bytes / redis_memory_max_bytes` | > 85% |
| `redis_connected_clients` | > 500（单节点） |
| `redis_keyspace_hits_total / (hits + misses)` 命中率 | < 80% |
| `redis_cluster_slots_fail` | > 0 |
