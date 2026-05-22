# Kafka 选型与用法

**版本要求：** ≥ 3.6（KRaft 模式，无 ZooKeeper）  
**部署模式：** 3 Broker + 3 Controller（KRaft combined mode 或 独立 Controller）

---

## 一、选型理由

| 特性 | 说明 |
|------|------|
| 高吞吐 | 单 Broker 百万 TPS，MaaS 峰值请求量可扛 |
| 持久化 + 可回放 | 审计日志、Trace 事件可按需重放消费 |
| KRaft 模式 | 去除 ZooKeeper 依赖，降低运维复杂度 |
| Consumer Group | 多服务独立消费同一 Topic，互不干扰 |
| Exactly Once | 幂等 Producer + 事务，防止计费重复 |

---

## 二、Topic 规划

| Topic | 分区数 | 副本 | 保留时长 | 生产者 | 消费者 |
|-------|--------|------|---------|-------|-------|
| `maas.request.audit` | 12 | 3 | 30天 | gateway-service | compliance-service、llmops-trace-service |
| `maas.trace.raw` | 24 | 3 | 7天 | gateway-service | llmops-trace-service |
| `maas.billing.events` | 12 | 3 | 30天 | gateway-service（结算触发） | billing-service |
| `maas.model.state` | 6 | 3 | 7天 | model-catalog-service | routing-service、adapter-service |
| `maas.routing.policy` | 6 | 3 | 7天 | routing-service | gateway-service |
| `maas.alert.p0` | 3 | 3 | 7天 | llmops-trace-service、billing-service | notification-service |
| `maas.alert.p1p2` | 6 | 3 | 3天 | 各服务 | notification-service |
| `maas.eval.result` | 6 | 3 | 14天 | prompt-eval-service | compliance-service |
| `maas.audit.compliance` | 6 | 3 | 90天 | compliance-service | — (冷归档到 MinIO) |

---

## 三、Schema 规范（Protobuf + Schema Registry）

所有 Topic 使用 Protobuf 序列化，通过 Confluent Schema Registry 管理版本。

```protobuf
// maas.trace.raw 消息结构（简化）
syntax = "proto3";
package maas.trace.v1;

message TraceEvent {
  string trace_id      = 1;
  string tenant_id     = 2;
  string model_id      = 3;
  int64  prompt_tokens = 4;
  int64  comp_tokens   = 5;
  int64  latency_ms    = 6;
  string status        = 7;
  int64  created_at_ms = 8;
}
```

---

## 四、Go Producer 配置

```go
import "github.com/IBM/sarama"

cfg := sarama.NewConfig()
cfg.Producer.RequiredAcks    = sarama.WaitForAll       // acks=all
cfg.Producer.Idempotent      = true                    // 幂等 Producer
cfg.Producer.Retry.Max       = 5
cfg.Producer.Compression     = sarama.CompressionLZ4
cfg.Net.SASL.Enable          = true
cfg.Net.SASL.Mechanism       = sarama.SASLTypeSCRAMSHA512
cfg.Net.SASL.User            = os.Getenv("KAFKA_USER")
cfg.Net.SASL.Password        = os.Getenv("KAFKA_PASSWORD")
cfg.Net.TLS.Enable           = true

producer, _ := sarama.NewSyncProducer(brokers, cfg)
```

---

## 五、Go Consumer 配置

```go
cfg := sarama.NewConfig()
cfg.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{
    sarama.NewBalanceStrategyRoundRobin(),
}
cfg.Consumer.Offsets.Initial    = sarama.OffsetNewest
cfg.Consumer.Offsets.AutoCommit.Enable   = false    // 手动提交
cfg.Consumer.Offsets.AutoCommit.Interval = 1 * time.Second

client, _ := sarama.NewConsumerGroup(brokers, "billing-service-group", cfg)

// 消费完成后手动标记
session.MarkMessage(msg, "")
```

---

## 六、幂等消费保障

所有消费者在处理业务逻辑前，先检查幂等键（通常为 `trace_id` 或 `event_id`）：

```go
// 使用 Redis SET NX 实现幂等
ok, _ := rdb.SetNX(ctx,
    fmt.Sprintf("maas:idem:%s", event.TraceId),
    "1", 5*time.Minute,
).Result()
if !ok {
    // 已处理，跳过
    session.MarkMessage(msg, "")
    return nil
}
// 执行业务逻辑...
```

---

## 七、监控指标

| 指标 | 告警阈值 |
|------|---------|
| Consumer Lag（`kafka_consumergroup_lag`） | > 10000 条（P0 Topic） |
| Broker Under-Replicated Partitions | > 0 |
| Producer Error Rate | > 0.1% |
| Disk Usage per Broker | > 80% |
