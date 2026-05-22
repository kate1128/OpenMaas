# Prometheus + Grafana 选型与用法

**Prometheus 版本：** ≥ 2.50  
**Grafana 版本：** ≥ 10.4  
**角色：** 指标采集、存储、可视化与告警

---

## 一、指标命名规范

```
maas_{service}_{metric_name}_{unit}
```

| 示例指标 | 类型 | 说明 |
|---------|------|------|
| `maas_gateway_requests_total` | Counter | 总请求数（按 tenant_id、model_id、status 标签） |
| `maas_gateway_request_duration_seconds` | Histogram | 请求延迟分布 |
| `maas_routing_policy_score` | Gauge | 路由策略当前评分 |
| `maas_billing_budget_remaining_yuan` | Gauge | 剩余预算（按租户） |
| `maas_adapter_cache_hit_total` | Counter | 语义缓存命中次数 |
| `maas_adapter_cache_miss_total` | Counter | 语义缓存未命中次数 |
| `maas_kafka_consumer_lag` | Gauge | Kafka 消费者 Lag |
| `maas_llm_tokens_total` | Counter | 累计 Token 消耗（按模型） |

---

## 二、服务暴露 /metrics（Go 示例）

```go
import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

var requestTotal = promauto.NewCounterVec(
    prometheus.CounterOpts{
        Name: "maas_gateway_requests_total",
        Help: "Total number of API requests",
    },
    []string{"tenant_id", "model_id", "status"},
)

var requestDuration = promauto.NewHistogramVec(
    prometheus.HistogramOpts{
        Name:    "maas_gateway_request_duration_seconds",
        Buckets: []float64{.05, .1, .25, .5, 1, 2.5, 5, 10},
    },
    []string{"model_id", "status"},
)

// 在 HTTP 路由注册
http.Handle("/metrics", promhttp.Handler())
```

---

## 三、Prometheus 抓取配置（prometheus.yml）

```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'maas-services'
    kubernetes_sd_configs:
      - role: pod
    relabel_configs:
      - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape]
        action: keep
        regex: 'true'
      - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_path]
        target_label: __metrics_path__
      - source_labels: [__meta_kubernetes_namespace, __meta_kubernetes_pod_name]
        separator: /
        target_label: instance

  - job_name: 'redis-exporter'
    static_configs:
      - targets: ['redis-exporter:9121']

  - job_name: 'kafka-exporter'
    static_configs:
      - targets: ['kafka-exporter:9308']

  - job_name: 'postgres-exporter'
    static_configs:
      - targets: ['pg-exporter:9187']
```

---

## 四、告警规则（PrometheusRule）

```yaml
groups:
  - name: maas-p0-alerts
    rules:
      - alert: HighErrorRate
        expr: |
          rate(maas_gateway_requests_total{status="error"}[5m])
          / rate(maas_gateway_requests_total[5m]) > 0.05
        for: 2m
        labels:
          severity: P0
        annotations:
          summary: "错误率超过 5%"

      - alert: BudgetExhausted
        expr: maas_billing_budget_remaining_yuan < 10
        for: 0m
        labels:
          severity: P0
        annotations:
          summary: "租户 {{ $labels.tenant_id }} 预算不足 10 元"

      - alert: KafkaConsumerLag
        expr: maas_kafka_consumer_lag{topic="maas.trace.raw"} > 10000
        for: 5m
        labels:
          severity: P1
        annotations:
          summary: "Trace Topic 消费积压超过 1 万条"
```

---

## 五、Grafana Dashboard 规划

| Dashboard | 数据源 | 说明 |
|-----------|--------|------|
| MaaS 全局总览 | Prometheus | QPS、P99延迟、错误率、Token消耗趋势 |
| 租户 FinOps | Prometheus + ClickHouse | 按租户成本、预算消耗、Token用量 |
| 模型健康看板 | Prometheus | 各模型可用率、健康评分、Fallback触发率 |
| 基础设施总览 | Prometheus | PG/Redis/Kafka/ClickHouse 资源使用 |
| Trace 分析 | ClickHouse | P99延迟分布、错误类型分析 |

所有 Dashboard JSON 模板存放在 `运维/grafana-dashboards/` 目录。
