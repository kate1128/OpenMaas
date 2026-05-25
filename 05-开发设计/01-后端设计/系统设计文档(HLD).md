# MaaS平台 系统高层设计文档 (High-Level Design)

**文档版本：** V2.0  
**编写日期：** 2026年05月25日  
**文档状态：** 初稿  
**关联PRD：** `产品设计/MaaS-PRD-V2.0/`  
**密级：** 内部

**变更说明：** V2.0 对齐 PRD V2.0：微服务从 11 个重构为 10 个；新增多协议端点支持（OpenAI/Anthropic/Gemini/WS）；强化合规安全架构（数据分级L0-L4 + Guardrails + ZDR + KMS）；模型管理升级为三层架构（ProviderModel→VendorBackend→LogicalModel）；语义缓存迁移至 adapter-service（Python原生）；引入 LLMOps Trace (ClickHouse) + Prompt评测中心 + 成本异常检测。

---

## 目录

1. [系统概述](#系统概述)
2. [设计原则与约束](#设计原则与约束)
3. [系统架构总览](#系统架构总览)
4. [核心子系统设计](#核心子系统设计)
5. [数据架构设计](#数据架构设计)
6. [部署架构设计](#部署架构设计)
7. [安全架构设计](#安全架构设计)
8. [非功能性需求设计](#非功能性需求设计)
9. [关键技术选型](#关键技术选型)
10. [系统接口设计](#系统接口设计)

---

## 1. 系统概述

### 1.1 系统目标

MaaS（模型即服务）平台是一个企业级大模型聚合与管理平台，旨在：

- 提供**多协议统一API网关**，同时兼容 OpenAI / Anthropic / Gemini 三套 API 标准，屏蔽多厂商模型差异
- 实现**三层模型架构**（ProviderModel → VendorBackend → LogicalModel）全生命周期管理
- 通过**智能路由引擎**以四维评分（成本 / 延迟 / 能力匹配 / 健康度）在成本与性能之间动态寻优
- 提供**精细化计量计费**（三优先级计量 + 四层预算 + 成本异常检测 + 节省建议引擎）
- 构建**企业级合规安全体系**（数据分级L0-L4 + 内容安全Guardrails + 零数据保留ZDR + 客户自带KMS + 审计哈希链）
- 支撑**LLMOps可观测性**（44字段Trace + Session视图 + 成本归因 + 异常聚类）
- 提供**Prompt实验与评测中心**（A/B实验 + LLM-as-Judge + 质量门禁 + 路由联动）

### 1.2 系统边界

**系统内部（In Scope）：**
- 多协议统一API网关（OpenAI/Anthropic/Gemini/WS）
- 三层模型架构（供应商模型→后端实例→逻辑模型）+ 模型目录市场工作台
- 智能路由引擎（四维评分 + Fallback链 + A/B联动 + 策略模拟）
- 语义缓存层（adapter-service Python原生）
- 计量计费系统（Append-Only账本 + FinOps仪表板）
- LLMOps观测（44字段Trace + Session视图 + 异常检测）
- Prompt实验与评测中心（A/B + LLM-as-Judge + 质量门禁）
- 合规安全服务（数据分级/Guardrails/ZDR/KMS/审计链）
- 多租户RBAC（19角色 + SSO/SCIM + 审批工作流）
- 告警通知中心（多通道 + 去重 + 值班路由）

**系统外部（Out of Scope）：**
- 大规模分布式模型全量训练平台
- 边缘模型管理与联邦学习
- 原生AI应用商店运营系统

---

## 2. 设计原则与约束

### 2.1 核心设计原则

| 原则 | 说明 |
|------|------|
| **高可用优先** | API网关SLA ≥ 99.95%，采用多活冗余架构 |
| **弹性伸缩** | 基于Kubernetes HPA/VPA，应对请求量突增 |
| **安全合规** | 零信任网络，全链路TLS 1.3，数据分级L0-L4，审计日志SHA-256哈希链防篡改 |
| **客户数据主权** | 支持客户自带KMS (BYOK)、零数据保留 (ZDR)、数据地域控制 |
| **可观测性** | Metrics / Tracing / Logging 三支柱，全栈可观测，44字段Trace |
| **多协议兼容** | 对外API同时兼容 OpenAI / Anthropic / Gemini 标准，协议自动检测与转换 |
| **数据驱动** | 埋点先行，指标可量化，路由策略可A/B验证，策略效果可模拟 |

---

## 3. 系统架构总览

### 3.1 整体分层架构（V2.0）

```
┌──────────────────────────────────────────────────────────────┐
│                    接入层 (Access Layer)                       │
│  Web控制台  │  CLI/SDK  │  OpenAI/Anthropic/Gemini REST  │ WS │
└─────────────────────────┬────────────────────────────────────┘
                          │ HTTPS / WSS
┌─────────────────────────▼────────────────────────────────────┐
│              统一API网关 gateway-service (Go)                   │
│  11步中间件链：认证/限流/合规前置/内容安全/路由分发/响应标准化    │
│  多协议检测（OpenAI/Anthropic/Gemini）→ 归一化 StandardRequest  │
└─────────────────────────┬────────────────────────────────────┘
                          │ gRPC路由决策 / HTTP流式直连
┌─────────────────────────▼────────────────────────────────────┐
│              智能路由引擎 routing-service (Go)                  │
│  四维评分 │ 5种策略类型 │ L1-L5 Fallback链 │ 策略模拟 │ A/B联动│
└──────┬─────────────────────────────────────────┬─────────────┘
       │ RouteDecision{backend_id}               │ 策略/端点查询
┌──────▼──────────┐                  ┌───────────▼─────────────┐
│ adapter-service │                  │ model-catalog-service    │
│  (Python 3.12)  │                  │      (Go)                │
│ LiteLLM协议翻译  │                  │ 三层模型架构              │
│ Key池+熔断+重试  │                  │ 供应商治理+市场工作台     │
│ 语义缓存(Python) │                  │ 健康评分+替换图谱         │
└──────┬──────────┘                  └──────────────────────────┘
       │ HTTP调用供应商API
┌──────▼────────────────────────────────────────────────────────┐
│              后端模型服务层 (Backend Models)                     │
│  外部API (OpenAI/Anthropic/DeepSeek/通义/文心/Gemini...)       │
│  自托管模型 (vLLM/TGI on GPU集群)                               │
└──────────────────────────────────────────────────────────────┘
                          │
┌─────────────────────────▼────────────────────────────────────┐
│              平台支撑服务层 (Platform Services)                  │
│  billing-service │ llmops-trace-service │ prompt-eval-service │
│  auth-service    │ compliance-service   │ notification-service│
└──────────────────────────────────────────────────────────────┘
                          │
┌─────────────────────────▼────────────────────────────────────┐
│              基础设施层 (Infrastructure)                        │
│  K8s│PostgreSQL│ClickHouse│Redis Cluster│Qdrant│Kafka│OSS/S3 │
└──────────────────────────────────────────────────────────────┘
```

### 3.2 请求处理主流程（V2.0）

```
开发者 → gateway(协议检测+认证+限流+合规预检+内容安全)
       → routing-service(四维评分 → RouteDecision{backend_id, fallback_chain})
       → adapter-service(LiteLLM协议翻译 → 供应商HTTP调用)
       → 响应 ← gateway(协议反向转换 + 注入x_maas扩展头) ← 开发者
       ↓（异步）
       Kafka(maas.api.requests) → billing-service(计量) + llmops-trace-service(Trace)
```

---

## 4. 核心子系统设计

### 4.1 统一API网关（gateway-service，Go）

**职责：** 作为平台唯一外部流量入口，负责多协议接入、认证、限流、合规前置、内容安全预检、路由分发、响应标准化。

**核心模块（11步中间件链）：**

| 步骤 | 模块 | 功能 |
|------|------|------|
| 0 | Trace注入 | 生成 trace_id / request_id，注入 gRPC metadata |
| 1 | 协议检测 | URL前缀识别协议（OpenAI/Anthropic/Gemini），归一化 StandardRequest |
| 2 | 认证 | API Key (HMAC-SHA256) + SSO JWT 双模认证 |
| 3 | 限流 | RPM/TPM 令牌桶，Key级/项目级/租户级三档限流 |
| 4 | 预算预检 | gRPC billing-service 余额查询，超额前置拒绝 |
| 5 | 合规前置 | gRPC compliance-service 数据驻留检查（ALLOW/DENY/REDIRECT） |
| 6 | 内容安全预检 | 本地轻量关键词过滤（P99 < 2ms） |
| 7 | 请求标准化 | JSON Schema校验，补全缺省字段，注入 zero_retention/kms_key_id |
| 8 | 路由分发 | gRPC routing-service 获取 RouteDecision |
| 9 | 供应商调用 | 同步：gRPC adapter；流式：HTTP直连 adapter |
| 10 | 响应标准化 | 反向协议转换 + 注入 x_maas 扩展头 |
| 11 | 事件埋点 | 异步发布至 Kafka maas.api.requests |

**关键接口（多协议）：**
- `POST /v1/chat/completions` — OpenAI 兼容
- `POST /v1/anthropic/messages` — Anthropic 兼容
- `POST /v1/gemini/generateContent` — Gemini 兼容
- `POST /v1/embeddings` — 文本嵌入
- `GET /v1/models` — 模型列表
- `POST /v1/ws` — WebSocket 流式代理

**非功能指标：**
- QPS处理能力：≥ 10,000 req/s（集群级别）
- 中间件链P99延迟（不含路由）：≤ 20ms
- 认证缓存命中率：≥ 95%

### 4.2 智能路由引擎（routing-service，Go）

**职责：** 根据路由策略、实时四维评分和A/B实验配置，将请求分发至最优 vendor_backend。

**四维评分公式（PRD §03）：**
```
Score_i = W1×cost_score + W2×latency_score + W3×capability_match + W4×health_score
```

**5种策略类型：** COST_OPTIMAL / PERFORMANCE / FIXED_MODEL / WEIGHTED_ROUND_ROBIN / CANARY

**Fallback链（L1→L5）：** 主后端 → 同供应商其他后端 → 跨供应商同档模型 → 跨供应商降级模型 → 缓存兜底

**A/B实验联动：** prompt-eval-service 创建 CANARY 策略 → routing 按比例分流 → 实验结论触发自动 promote/rollback

**质量门禁联动：** eval结果 → quality_score更新 → Kafka事件 → routing自动调整权重

详见 `微服务设计/02-routing-service详细设计.md`。

### 4.3 语义缓存层（adapter-service 内，Python）

**架构决策（V2.0）：** 语义缓存归入 adapter-service（Python原生实现），消除跨语言IPC开销。

**缓存架构：**
- Embedding：bge-m3 轻量模型，计算 prompt embedding
- 向量存储：Qdrant（大规模）/ Redis Vector（小规模），cosine ≥ 0.92 命中
- 适用条件：temperature=0 + tools为空 + 租户 cache_enabled=true

**事件驱动失效：** 消费 Kafka maas.model.events → 模型生命周期变更/定价变更 → 针对性清除对应 logical_model_id / tenant_id 缓存

### 4.4 模型目录管理（model-catalog-service，Go）

**三层模型架构（PRD §02）：**
```
ProviderModel (供应商原始模型)
  └── VendorBackend (具体后端实例，含价格/健康/Key池)
        └── LogicalModel (面向租户的逻辑模型，含能力标签/合规标签/基准评测)
```

**50+字段模型卡片：** 模态支持 / 上下文窗口 / Tool Calling / Streaming / JSON模式 / Embedding / 微调支持 / 数据驻留区域 / PII处理 / 质量等级 / 基准评测分数 / 弃用通知 / 迁移指南 / 市场可见性 等。

**模型生命周期状态机（PRD §02 5.1）：**
```
draft → testing → canary → active → paused → deprecated → retired
```

**供应商治理：** 供应商入驻审核 / SLA协议 / 合同管理 / Key池管理（配额预测 + 健康评分5维算法）/ 替换图谱（任务范围 + 兼容性评分 + 置信度）

### 4.5 计量计费引擎（billing-service，Go）

**三优先级计量（PRD §06 1.2）：** 供应商usage字段 > 网关Tokenizer > 字符估算(字符数/4)

**预扣-核销两阶段：** 请求前 gRPC 余额预检（PreCheckQuota） → 请求后 Kafka 消费核销

**四层预算（PRD §06 第2章）：** 平台级 → 租户级 → 项目级 → Key级，硬限拒绝/软限告警

**三种异常检测算法（PRD §06 第4章）：** Z-Score（实时） + Prophet时间序列分解（每日批处理） + KL散度（分布漂移）

**节省建议引擎（PRD §06 第5章）：** Prompt Cache建议 / 模型替换 / Batch API / Token优化 / Key回收

详见 `微服务设计/05-billing-service详细设计.md`。

---

## 5. 数据架构设计

### 5.1 数据存储选型（V2.0）

| 数据类型 | 存储方案 | 理由 |
|---------|---------|------|
| 业务主数据（用户、项目、模型元数据） | PostgreSQL | ACID事务，关系完整性，RLS数据隔离 |
| 账单数据 | PostgreSQL（独立实例） | Append-Only，强一致性，单独备份 |
| Trace / LLMOps时序数据 | ClickHouse | 44字段宽表，高写入吞吐，列式压缩 |
| 语义向量缓存 | Qdrant / Redis Vector | ANN向量检索，payload过滤 |
| 缓存 / 限流 / 会话 | Redis Cluster | 高性能键值，支持TTL / Lua脚本 |
| 消息队列 | Apache Kafka | 日志/事件解耦，高吞吐，多消费者组 |
| 对象存储（数据集/模型文件） | MinIO / OSS | 大文件存储 |
| 日志 | Elasticsearch + Kibana | 全文检索，大规模日志查询 |

### 5.2 核心数据实体关系（概念层 — V2.0三层模型）

```
Tenant（租户）
  ├── User（用户）+ Roles（19角色RBAC）
  │    └── ApiKey（5种类型：mk-prod/mk-dev/mk-ci/mk-pg/mk-sub）
  ├── Project（项目）
  │    ├── RoutingPolicy（路由策略：30+字段）
  │    └── BudgetConfig（四层预算）
  ├── Vendor（供应商：30+字段）
  │    ├── VendorContract（合同管理）
  │    ├── VendorSlaEvent（SLA事件）
  │    └── VendorBackend（后端实例：含价格/健康评分/Key池）
  │         ├── VendorKey（Key池：含加密存储/健康度/配额预测）
  │         └── ProviderModel（供应商原始模型）
  ├── LogicalModel（逻辑模型：50+字段模型卡片）
  │    └── ModelReplacementGraph（模型替换图谱）
  └── BillingStatement（账单）
       └── BillingLedger（34字段 Append-Only 账本）
```

---

## 6. 部署架构设计

### 6.1 生产环境部署拓扑

```
公网流量 → CDN/WAF → 负载均衡器 (LB)
                         │
         ┌───────────────┼───────────────┐
         ▼               ▼               ▼
    API网关Pod1      API网关Pod2      API网关PodN
         └───────────────┼───────────────┘
                         │（K8s Service）
         ┌───────────────┼───────────────┐
         ▼               ▼               ▼
    路由引擎Pod1     路由引擎Pod2    路由引擎PodN
                         │
         ┌───────────────┼───────────────┐
         ▼               ▼               ▼
    模型适配器集群    缓存服务集群    计费服务集群
                         │
    ┌────────────────────┴────────────────────┐
    ▼                                         ▼
GPU推理集群（自托管模型）           外部模型API（云厂商）
```

### 6.2 高可用设计

| 组件 | 冗余方案 | RTO | RPO |
|------|---------|-----|-----|
| API网关 | 多Zone双活，≥3副本 | < 30s | 0 |
| 路由引擎 | 无状态，多副本 | < 30s | 0 |
| PostgreSQL | 主从复制 + 自动Failover | < 60s | < 5s |
| Redis | Cluster模式，3主3从 | < 30s | 0（内存） |
| Kafka | 多副本分区，RF=3 | < 60s | 接近0 |

### 6.3 弹性伸缩策略

- **API网关**：HPA基于CPU使用率（> 70%扩容），最小3副本，最大20副本
- **路由引擎**：HPA基于请求队列长度，响应时间P95超阈值触发扩容
- **GPU推理节点**：基于GPU使用率 + 请求队列，异构节点池调度（NVIDIA优先，昇腾备用）

---

## 7. 安全架构设计（PRD §07）

### 7.1 安全分层防护（四层合规架构）

```
L1 策略层：数据分级（L0-L4）、地域控制、Guardrails策略、合规审批工作流（双人审批）
L2 执行层：网关预过滤、内容安全引擎（实时检测）、路由地域过滤、KMS加密执行
L3 记录层：Append-Only审计日志、SHA-256哈希链防篡改、外部审计集成（Splunk/SIEM）
L4 证明层：合规报告包（可机读+可人读）、审计日志加密导出、策略变更全版本历史
```

### 7.2 内容安全机制（Guardrails — PRD §07 第3章）

**三层检测引擎（三个执行位置）：**

| 执行位置 | 检测类型 | 延迟目标 |
|---------|---------|---------|
| Pre-Route（网关层） | 关键词过滤 + PII Regex 快速扫描 | P99 < 5ms |
| Pre-Dispatch（路由层） | 语义分类 + Prompt注入检测 + 话题边界检测 | P99 < 30ms |
| Post-Response（响应层） | 输出合规检测 + 版权检测 + 免责声明注入 | P99 < 20ms |

**8种风险类型：** 极高风险/违禁话题/PII泄露/Prompt注入/话题边界超出/不当专业建议/商业敏感/版权内容

**PII检测两阶段混合策略：** Regex快速扫描(P99<1ms) + BERT-CRF NER模型(P99<50ms, Precision>95%)

### 7.3 零数据保留模式（ZDR — PRD §07 第4章）

两个模式：
- **Metadata-Only**：仅保留元数据（request_id/token量/延迟/成本等），不保留Prompt/Response内容
- **True Zero Retention**：连元数据关联关系也不保留，仅保留加密计费凭证

ZDR标记通过 gRPC metadata `x-maas-zero-retention: 1` 全链路传播。

### 7.4 客户自带KMS（BYOK — PRD §07 第5章）

支持 AWS KMS / 阿里云 KMS / HashiCorp Vault。客户保管密钥，平台保管密文。

---

## 8. 非功能性需求设计

### 8.1 性能目标

| 指标 | 目标值 | 测量方式 |
|------|--------|---------|
| API网关可用性 | ≥ 99.95% | Uptime监控 |
| 请求P95延迟（不含模型推理） | ≤ 50ms | APM追踪 |
| 请求P95延迟（含模型推理） | ≤ 800ms | 端到端监控 |
| 峰值QPS（集群） | ≥ 10,000 | 压力测试 |
| 语义缓存命中率 | ≥ 20% | 埋点统计 |
| GPU资源利用率 | ≥ 70% | 基础设施监控 |

### 8.2 可观测性设计

**三支柱体系：**

| 支柱 | 工具 | 用途 |
|------|------|------|
| **Metrics** | Prometheus + Grafana | 系统与业务指标实时监控 |
| **Tracing** | Jaeger / OpenTelemetry | 分布式链路追踪，定位跨服务延迟 |
| **Logging** | ELK Stack（ES+Logstash+Kibana） | 日志聚合、查询、告警 |

**告警层级：**

| 等级 | 触发条件示例 | 通知方式 | 响应时间 |
|------|-------------|---------|---------|
| P0 - 严重 | 网关不可用 / 数据泄露 | 电话 + 短信 + 钉钉 | 15分钟内 |
| P1 - 高 | 错误率 > 1% / 延迟P99 > 3s | 短信 + 钉钉 | 30分钟内 |
| P2 - 中 | 缓存命中率骤降 / 预算告警 | 钉钉 + 邮件 | 2小时内 |
| P3 - 低 | 日报异常 / 容量预测告警 | 邮件 | 下个工作日 |

---

## 9. 关键技术选型

| 层次 | 组件 | 选型 | 选型理由 |
|------|------|------|---------|
| API网关 | 流量接入 | Nginx + 自研网关服务 | 高性能，可扩展，支持插件 |
| 路由引擎 | 核心服务 | Go（高并发，低延迟） | GC开销低，并发模型优异 |
| 模型管理 | 推理框架 | vLLM / TGI | 高吞吐量推理，PagedAttention优化 |
| 精调框架 | 微调工具 | LLaMA-Factory / PEFT | 支持LoRA/QLoRA，多模型适配 |
| 容器编排 | 调度平台 | Kubernetes + GPU Operator | 行业标准，支持GPU异构调度 |
| 消息队列 | 异步解耦 | Apache Kafka | 高吞吐，持久化，多消费者组 |
| 向量数据库 | 语义缓存 | Milvus | 开源，高性能ANN检索 |
| 前端框架 | 控制台 | React + TypeScript | 生态成熟，组件库丰富 |
| CI/CD | 流水线 | GitLab CI + ArgoCD | GitOps，K8s原生部署 |

---

## 10. 系统接口设计

### 10.1 外部接口（对开发者 — 多协议）

| 接口 | 方法 | 协议 | 说明 |
|------|------|------|------|
| `/v1/chat/completions` | POST | OpenAI | 对话补全（HTTP/SSE） |
| `/v1/embeddings` | POST | OpenAI | 文本嵌入向量 |
| `/v1/models` | GET | OpenAI | 可用逻辑模型列表 |
| `/v1/anthropic/messages` | POST | Anthropic | Messages API |
| `/v1/anthropic/messages/stream` | POST | Anthropic | 流式 Messages（SSE） |
| `/v1/gemini/generateContent` | POST | Gemini | 内容生成 |
| `/v1/gemini/streamGenerateContent` | POST | Gemini | 流式生成（SSE） |
| `/v1/ws` | WS | WebSocket | 流式对话代理 |
| `/healthz` | GET | - | K8s 探针 |

### 10.2 内部服务间接口

- **网关 → 路由引擎**：gRPC `RouteRequest(StandardRequest) → RouteDecision{backend_id, fallback_chain}`
- **网关 → adapter（流式）**：HTTP POST `/v1/chat/completions/stream`（直接HTTP，避免SSE穿越gRPC）
- **网关 → adapter（同步）**：gRPC `CallVendorBackend(VendorCallRequest) → VendorCallResponse`
- **adapter → 后端模型**：LiteLLM 自动协议转换，HTTP调用供应商原生API
- **各服务 → Kafka**：异步事件发布
- **各服务 → Prometheus**：Pull模式暴露 `/metrics`

### 10.3 外部系统集成接口

| 外部系统 | 集成方式 | 数据内容 |
|---------|---------|---------|
| 公司SSO系统 | SAML2 / OIDC | 用户认证与身份同步（SCIM） |
| 财务ERP | REST API + 定时同步 | 账单数据、费用归因 |
| 基础设施算力平台 | K8s API + 自定义Operator | GPU资源调度与监控 |
| 钉钉/企业微信/PagerDuty | Webhook | 告警通知推送 |
| 客户KMS | AWS KMS / 阿里云KMS / Vault API | 密钥管理、加解密操作 |
| SIEM系统 | Syslog / Kafka | 审计日志投递（Splunk等） |

---

## 变更历史

| 版本 | 日期 | 修改内容 | 修改人 |
|------|------|---------|--------|
| V1.0 | 2026-05-14 | 初始版本，基于PRD V4.0生成 | — |
| V2.0 | 2026-05-25 | 对齐 PRD V2.0：10服务架构、多协议端点、11步中间件链、三层模型架构、四维路由评分、四层合规安全（Guardrails/ZDR/KMS）、LLMOps + 评测中心、事件驱动缓存失效 | — |
