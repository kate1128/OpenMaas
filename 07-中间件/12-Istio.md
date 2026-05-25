# Istio 选型与用法

**版本要求：** ≥ 1.20（含 Ambient Mesh 模式）  
**角色：** 服务网格 — 服务间流量管理、mTLS 通信加密、灰度发布、熔断/重试、可观测性注入

---

## 一、选型理由

| 特性 | 说明 |
|------|------|
| 零代码侵入 | Sidecar 注入（或 Ambient 无 Sidecar 模式），业务容器无需修改代码即可获得流量治理能力 |
| 精细流量管理 | VirtualService + DestinationRule 支持权重/Header 灰度、镜像流量、超时/重试/熔断策略 |
| 自动化 mTLS | 服务间全量 mTLS 加密，无需业务层处理证书，PeerAuthentication 策略按命名空间控制 |
| 可观测性注入 | Envoy 自动上报 HTTP/gRPC 指标到 Prometheus，分布式 Trace 随请求传播，无需业务 SDK |
| 多集群支持 | 通过 Primary-Remote 架构管理多个 K8s 集群，统一服务网格边界 |
| 生态成熟 | CNCF 毕业项目，与 Prometheus/Kiali/Jaeger/Grafana 深度集成 |

### 为什么不选 Linkerd / Consul Mesh

| 替代方案 | 劣势 |
|---------|------|
| Linkerd | 无 VirtualService/DestinationRule 级别的流量路由能力，无法实现精细化灰度发布；功能集远小于 Istio |
| Consul Mesh | 与 Consul 强绑定，运维复杂度高；K8s 集成不如 Istio 原生，社区活跃度低 |

---

## 二、架构角色

```
┌─────────────────────────────────────────────────────────────────────┐
│                        K8s Cluster                                  │
│                                                                     │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐          │
│  │ gateway-svc  │    │ routing-svc  │    │ billing-svc  │  ...      │
│  │   (Sidecar)  │    │   (Sidecar)  │    │   (Sidecar)  │          │
│  └──────┬───────┘    └──────┬───────┘    └──────┬───────┘          │
│         │                   │                   │                   │
│         └───────────────────┼───────────────────┘                   │
│                             │                                       │
│                    ┌────────▼────────┐                              │
│                    │   Istiod         │                             │
│                    │ (控制面 Pilot)   │                             │
│                    │ XDS → Envoy     │                              │
│                    └─────────────────┘                              │
│                                                                     │
│  Ingress Gateway (Envoy) ← Nginx ← Internet                        │
└─────────────────────────────────────────────────────────────────────┘
```

### 组件说明

| 组件 | 角色 |
|------|------|
| Istiod | 控制面：Pilot（XDS 下发）、Citadel（证书签发）、Galley（配置校验） |
| Envoy Sidecar | 数据面：每个 Pod 旁挂的 L7 代理，拦截出入流量执行策略 |
| Ingress Gateway | 集群入口 Gateway，替代部分 Nginx 职责处理南北向流量（可选） |
| Egress Gateway | 集群出口 Gateway，管控对外部服务的访问策略（可选） |

---

## 三、核心配置

### 3.1 启用 Sidecar 注入

为命名空间开启自动注入：

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: maas
  labels:
    istio-injection: enabled   # 自动注入 Envoy Sidecar
```

或通过 Revision 指定控制面版本（金丝雀升级用）：

```yaml
istio.io/rev: canary
```

### 3.2 VirtualService + DestinationRule — 灰度发布

以 routing-service 为例，将 10% 流量切到新版本：

```yaml
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: routing-service
spec:
  hosts:
    - routing-service
  http:
    - match:
        - headers:
            x-canary:
              exact: "true"          # Header 匹配：优先路由 v2
      route:
        - destination:
            host: routing-service
            subset: v2
    - route:
        - destination:
            host: routing-service
            subset: v1
          weight: 90
        - destination:
            host: routing-service
            subset: v2
          weight: 10                 # 权重灰度：10% 流量到 v2
---
apiVersion: networking.istio.io/v1beta1
kind: DestinationRule
metadata:
  name: routing-service
spec:
  host: routing-service
  subsets:
    - name: v1
      labels:
        version: v1
    - name: v2
      labels:
        version: v2
  trafficPolicy:
    connectionPool:
      tcp:
        maxConnections: 100
      http:
        http1MaxPendingRequests: 1024
        maxRequestsPerConnection: 10
    loadBalancer:
      simple: LEAST_CONN
    outlierDetection:                # 熔断
      consecutive5xxErrors: 5
      interval: 30s
      baseEjectionTime: 60s
```

### 3.3 自动化 mTLS

全局启用到 STRICT 模式：

```yaml
apiVersion: security.istio.io/v1beta1
kind: PeerAuthentication
metadata:
  name: default
  namespace: maas
spec:
  mtls:
    mode: STRICT                    # 强制 mTLS，拒绝明文流量
```

如需允许某些服务使用明文（如监控探针）：

```yaml
apiVersion: security.istio.io/v1beta1
kind: PeerAuthentication
metadata:
  name: permissive-monitoring
  namespace: maas
spec:
  selector:
    matchLabels:
      app: prometheus
  mtls:
    mode: PERMISSIVE
```

### 3.4 熔断与重试

adapter-service 调用外部 LLM API 时配置熔断：

```yaml
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: adapter-service-outbound
spec:
  hosts:
    - api.openai.com
  tls:
    - match:
        - port: 443
          sniHosts:
            - api.openai.com
      route:
        - destination:
            host: api.openai.com
            port:
              number: 443
  http:
    - retries:
        attempts: 3
        perTryTimeout: 30s
        retryOn: gateway-error,connect-failure,refused-stream
      timeout: 60s
```

---

## 四、灰度发布工作流

```
Dev 提交 v2 代码 → CI 构建镜像 → 部署 routing-service:v2
    │
    ▼
创建 DestinationRule 定义 v1/v2 subsets
    │
    ▼
更新 VirtualService: v2 weight=10%
    │
    ▼
监控 Istio 指标 → 请求成功率、P99 延迟、错误率
    │
    ▼
确认稳定 → weight 逐步提升 10% → 50% → 100%
    │
    ▼
v1 下线 → 清理 v1 Deployment 和 DestinationRule subset
```

---

## 五、可观测性集成

Istio 自动为每个 Sidecar 生成指标，无需业务代码修改。

### 5.1 关键指标

| 指标 | 来源 | 告警场景 |
|------|------|---------|
| `istio_requests_total` | Envoy | 服务间请求量突降 → 可能熔断 |
| `istio_request_duration_milliseconds` | Envoy | P99 > 5s → 慢服务 |
| `istio_requests_total{response_code="5xx"}` | Envoy | 5xx 率 > 5% → 故障 |
| `istio_tcp_sent_bytes_total` | Envoy | 流量异常突增 → 可能 DDoS |

### 5.2 Kiali 可视化

```
Kiali Dashboard → Service Graph → 服务间拓扑、健康状态、流量比例
```

Kiali 展示 Service Graph，实时显示服务间调用关系、协议、延迟、错误率。灰度发布时可直观看到流量比例变化。

### 5.3 Trace 传播

Envoy Sidecar 自动传播 `x-request-id` 和 Trace Header，配合 OTel Collector 采集：

```
请求 → Ingress Gateway (Envoy) → gateway-service → routing-service → adapter-service
                                      ↓                  ↓                  ↓
                                 Envoy 注入 Span → OTel Collector → Jaeger
```

---

## 六、与 Nginx 的分工

| 层次 | 组件 | 职责 |
|------|------|------|
| 南北向流量（边缘） | Nginx | TLS 终止、域名路由、IP 级别限流、静态文件服务 |
| 东西向流量（服务间） | Istio Sidecar | 服务间 mTLS、灰度路由、熔断重试、可观测注入 |

Nginx 与 Istio 配合实现「边缘安全 + 服务网格」分层治理：Nginx 负责入口防御，Istio 负责内部流量精细管控。

---

## 七、多集群部署（规划）

生产环境规划 2 个 K8s 集群（主备 / 同城双活），Istio 多集群架构：

```
┌──────────────┐         ┌──────────────┐
│  Cluster A   │         │  Cluster B   │
│ (Primary)    │◄──XDS──►│ (Remote)     │
│   Istiod     │         │              │
│   Envoy      │         │   Envoy      │
└──────────────┘         └──────────────┘
```

通过 Istio 多集群网络，实现跨集群服务发现和流量路由。

---

## 八、运维要点

| 场景 | 操作 |
|------|------|
| Sidecar 资源超卖 | 设置 Sidecar 资源 `resources.requests` 避免 OOM；`proxy.istio.io/config` 调整并发连接数 |
| 升级控制面 | 使用 Revision 机制部署新版本 Istiod，逐个命名空间切换 |
| 排查 Envoy 问题 | `istioctl proxy-status` 查看 XDS 同步状态；`istioctl proxy-config` 查看 Envoy 配置 |
| 关闭 Sidecar 注入 | `kubectl label namespace maas istio-injection-` 并重启 Pod |

---

## 九、监控告警（Prometheus）

| 规则 | 表达式 | 级别 |
|------|--------|------|
| Sidecar 无响应 | `rate(istio_requests_total{response_code="503"}[5m]) > 0.1` | P1 |
| 高延迟服务 | `histogram_quantile(0.99, ... istio_request_duration_milliseconds ...) > 5000` | P1 |
| 灰度流量异常 | 对比 v1/v2 的 `istio_requests_total` 和错误率 | P2 |
