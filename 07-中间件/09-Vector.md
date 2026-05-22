# Vector 选型与用法

**版本要求：** ≥ 0.38  
**角色：** 日志聚合、转换与路由管道

---

## 一、选型理由

| 特性 | 说明 |
|------|------|
| Rust 实现 | 零拷贝内存模型，CPU/内存占用远低于 Logstash |
| VRL（Vector Remap Language） | 强类型日志转换 DSL，表达力强 |
| Kafka Source/Sink | 直接消费/生产 Kafka，适合 MaaS 日志流 |
| 多 Sink 扇出 | 同一日志流可同时写入 ClickHouse、MinIO、标准输出 |
| 背压控制 | 内置 Buffer + 背压，防止下游阻塞导致日志丢失 |

---

## 二、部署模式

```
Pod（每节点 DaemonSet）
  └─ Vector Agent（采集 stdout/stderr）
        │ Vector Protocol / Kafka
        ▼
  Vector Aggregator（集中式处理）
        ├─ Sink: Kafka maas.audit.compliance（审计日志）
        ├─ Sink: ClickHouse llm_traces（结构化 Trace 日志）
        └─ Sink: MinIO maas-audit-logs（冷归档）
```

---

## 三、Vector Agent 配置（agent-config.yaml）

```toml
# 采集 Kubernetes Pod 日志
[sources.k8s_logs]
type = "kubernetes_logs"
namespace_labels = ["maas-*"]
auto_partial_merge = true

# 解析 JSON 日志
[transforms.parse_json]
type = "remap"
inputs = ["k8s_logs"]
source = '''
  . = parse_json!(string!(.message))
  .pod_name = del(.kubernetes.pod_name)
  .namespace = del(.kubernetes.namespace)
'''

# 过滤非业务日志
[transforms.filter_business]
type = "filter"
inputs = ["parse_json"]
condition = '.level != "debug" && exists(.trace_id)'

[sinks.kafka_audit]
type = "kafka"
inputs = ["filter_business"]
bootstrap_servers = "kafka-0:9092,kafka-1:9092"
topic = "maas.request.audit"
encoding.codec = "json"
```

---

## 四、Vector Aggregator 配置（VRL 转换示例）

```toml
[sources.kafka_trace]
type = "kafka"
bootstrap_servers = "kafka-0:9092"
group_id = "vector-aggregator"
topics = ["maas.trace.raw"]
decoding.codec = "json"

# VRL：字段标准化 + 类型转换
[transforms.normalize_trace]
type = "remap"
inputs = ["kafka_trace"]
source = '''
  .latency_ms = to_int!(.latency_ms)
  .total_tokens = to_int!(.prompt_tokens) + to_int!(.completion_tokens)
  .cost_usd = to_float!(.cost_usd)
  .created_at = parse_timestamp!(.created_at, format: "%+")
  # 过滤零 Token 异常记录
  assert!(to_int(.total_tokens) > 0, "zero token record dropped")
'''

# 写入 ClickHouse
[sinks.clickhouse]
type = "clickhouse"
inputs = ["normalize_trace"]
endpoint = "http://clickhouse:8123"
database = "maas"
table = "llm_traces"
batch.max_events = 2000
batch.timeout_secs = 1
compression = "gzip"

# 冷归档到 MinIO（Parquet 格式）
[sinks.minio_archive]
type = "aws_s3"
inputs = ["normalize_trace"]
bucket = "maas-trace-archive"
endpoint = "http://minio:9000"
region = "us-east-1"
key_prefix = "traces/%Y/%m/%d/"
encoding.codec = "parquet"
batch.max_bytes = 134217728   # 128MB 滚动
```

---

## 五、监控 Vector 自身

Vector 暴露 Prometheus 指标（端口 9598）：

| 指标 | 说明 |
|------|------|
| `vector_events_processed_total` | 已处理事件总数 |
| `vector_buffer_events` | Buffer 积压事件数 |
| `vector_component_errors_total` | 组件错误数 |
| `vector_sink_send_errors_total` | Sink 发送失败数 |

告警：`vector_buffer_events > 50000` 触发 P1 告警。
