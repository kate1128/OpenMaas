# Qdrant 选型与用法

**版本要求：** ≥ 1.9  
**角色：** 语义向量检索（adapter-service 语义缓存）

---

## 一、选型理由

| 特性 | 说明 |
|------|------|
| 纯向量数据库 | 专为向量检索优化，HNSW 索引，延迟 < 5ms |
| Payload 过滤 | 支持向量检索 + 结构化字段联合过滤（tenant_id、model_id） |
| gRPC API | 高性能二进制协议，适合微服务间调用 |
| Rust 实现 | 内存安全，无 GC 抖动 |
| 磁盘存储 | 支持 mmap，大规模向量集无需全量内存 |

---

## 二、Collection 设计

```json
// 创建语义缓存 Collection
PUT /collections/semantic_cache
{
  "vectors": {
    "size": 1024,
    "distance": "Cosine",
    "hnsw_config": {
      "m": 16,
      "ef_construct": 100,
      "full_scan_threshold": 10000
    },
    "on_disk": true
  },
  "payload_schema": {
    "tenant_id": { "data_type": "keyword" },
    "model_id":  { "data_type": "keyword" },
    "created_at":{ "data_type": "integer" }
  },
  "optimizers_config": {
    "deleted_threshold": 0.2,
    "vacuum_min_vector_number": 1000
  }
}
```

---

## 三、写入向量点

```go
import "github.com/qdrant/go-client/qdrant"

client, _ := qdrant.NewClient(&qdrant.Config{
    Host: "qdrant",
    Port: 6334,   // gRPC 端口
})

// 写入缓存点
client.Upsert(ctx, &qdrant.UpsertPoints{
    CollectionName: "semantic_cache",
    Points: []*qdrant.PointStruct{
        {
            Id:      qdrant.NewIDNum(hashID),
            Vectors: qdrant.NewVectors(embedding...),
            Payload: qdrant.NewValueMap(map[string]any{
                "tenant_id":  tenantID,
                "model_id":   modelID,
                "response":   responseText,
                "created_at": time.Now().Unix(),
            }),
        },
    },
})
```

---

## 四、语义检索（混合过滤 + KNN）

```go
results, _ := client.Query(ctx, &qdrant.QueryPoints{
    CollectionName: "semantic_cache",
    Query: qdrant.NewQuery(embedding...),
    Limit: qdrant.PtrOf(uint64(1)),
    ScoreThreshold: qdrant.PtrOf(float32(0.92)),  // 余弦相似度阈值
    Filter: &qdrant.Filter{
        Must: []*qdrant.Condition{
            qdrant.NewMatchKeyword("tenant_id", tenantID),
            qdrant.NewMatchKeyword("model_id", modelID),
        },
    },
    WithPayload: qdrant.NewWithPayload(true),
})

if len(results) > 0 && results[0].Score >= 0.92 {
    return results[0].Payload["response"]  // 命中缓存
}
```

---

## 五、TTL 清理（定期删除过期点）

Qdrant 本身不支持 TTL，通过定时任务清理：

```go
// 每小时清理 1 小时前的缓存点
expireAt := time.Now().Add(-1 * time.Hour).Unix()
client.Delete(ctx, &qdrant.DeletePoints{
    CollectionName: "semantic_cache",
    Points: qdrant.NewPointsSelectorFilter(&qdrant.Filter{
        Must: []*qdrant.Condition{
            qdrant.NewRange("created_at", nil, &expireAt, nil, nil),
        },
    }),
})
```

---

## 六、部署规格

| 参数 | 值 |
|------|---|
| 副本 | 1（开发）/ 3（生产） |
| HTTP 端口 | 6333 |
| gRPC 端口 | 6334 |
| 内存（单节点） | ≥ 4GB（向量数 < 500万） |
| 存储 | SSD，on_disk=true 时向量落盘 |
| Shard 数 | 2（生产集群） |
