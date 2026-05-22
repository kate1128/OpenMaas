# OpenTelemetry + Jaeger 选型与用法

**OTel Collector 版本：** ≥ 0.100  
**Jaeger 版本：** ≥ 1.57  
**角色：** 分布式 Trace 采集、传输与查询

---

## 一、选型理由

| 特性 | 说明 |
|------|------|
| CNCF 标准 | OpenTelemetry 是可观测性事实标准，SDK 覆盖 Go/Python |
| 供应商无关 | Collector 作为中间层，后端可切换 Jaeger/Tempo/Zipkin |
| 自动插桩 | go.opentelemetry.io/contrib/instrumentation 自动为 HTTP/gRPC 添加 Span |
| Jaeger OTLP | Jaeger 原生支持 OTLP 接收，无需额外转换 |
| 采样控制 | Tail-based sampling 按错误率动态采样，控制存储成本 |

---

## 二、架构流程

```
服务 (OTel SDK)
    │ OTLP/gRPC :4317
    ▼
OTel Collector（DaemonSet）
    ├─ Processor: batch(512, 5s) + memory_limiter
    ├─ Exporter → Jaeger Collector :14250 (OTLP)
    └─ Exporter → Kafka maas.trace.raw (自定义 Span → TraceEvent 转换)
```

---

## 三、Go SDK 初始化

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
    "go.opentelemetry.io/otel/sdk/trace"
    "go.opentelemetry.io/otel/sdk/resource"
    semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

func initTracer(serviceName string) func() {
    exporter, _ := otlptracegrpc.New(ctx,
        otlptracegrpc.WithEndpoint("otel-collector:4317"),
        otlptracegrpc.WithInsecure(),
    )

    tp := trace.NewTracerProvider(
        trace.WithBatcher(exporter,
            trace.WithMaxExportBatchSize(512),
            trace.WithBatchTimeout(5*time.Second),
        ),
        trace.WithSampler(trace.ParentBased(trace.TraceIDRatioBased(0.1))), // 10% 采样
        trace.WithResource(resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceName(serviceName),
            semconv.ServiceVersion("2.0.0"),
            attribute.String("maas.env", os.Getenv("ENV")),
        )),
    )
    otel.SetTracerProvider(tp)
    return func() { tp.Shutdown(ctx) }
}
```

---

## 四、Span 属性规范（MaaS 扩展）

| 属性键 | 类型 | 说明 |
|--------|------|------|
| `maas.tenant_id` | string | 租户 ID |
| `maas.model_id` | string | 逻辑模型 ID |
| `maas.vendor_id` | string | 实际调用的供应商 |
| `maas.prompt_tokens` | int | Prompt Token 数 |
| `maas.completion_tokens` | int | Completion Token 数 |
| `maas.cost_usd` | float | 本次请求成本 |
| `maas.cache_hit` | bool | 语义缓存是否命中 |
| `maas.route_policy_id` | string | 命中的路由策略 ID |
| `maas.fallback_level` | int | Fallback 层级（0=未降级） |

---

## 五、OTel Collector 配置（otelcol-config.yaml）

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: "0.0.0.0:4317"
      http:
        endpoint: "0.0.0.0:4318"

processors:
  memory_limiter:
    check_interval: 1s
    limit_mib: 512
  batch:
    send_batch_size: 512
    timeout: 5s
  resource:
    attributes:
      - action: insert
        key: deployment.environment
        value: ${ENV}

exporters:
  otlp/jaeger:
    endpoint: "jaeger-collector:4317"
    tls:
      insecure: true
  kafka:
    brokers: ["kafka-0:9092", "kafka-1:9092"]
    topic: "maas.trace.raw"
    encoding: "otlp_proto"

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [memory_limiter, batch, resource]
      exporters: [otlp/jaeger, kafka]
```

---

## 六、采样策略

| 场景 | 采样率 | 说明 |
|------|--------|------|
| 正常请求 | 10% | TraceIDRatioBased(0.1) |
| 错误请求 | 100% | AlwaysSample for status=error |
| P0 租户 | 100% | 头部采样注入 `x-sample-rate: always` |

---

## 七、Jaeger 查询示例

```
# 查询某租户最近 1 小时慢请求（> 2s）
service=gateway-service
tags: maas.tenant_id=xxx, maas.model_id=gpt-4o
minDuration: 2s
```
