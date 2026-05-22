# MaaS平台 PRD V2.0 —— 第04章：LLMOps观测与请求Trace规格

**文档版本：** V2.0.0  
**编写日期：** 2026年05月21日  
**文档状态：** 设计评审中  
**机密等级：** 内部保密  
**所属模块：** LLMOps观测体系  
**前置文档：** `03-路由策略与容灾降级规格.md`  
**后续文档：** `05-Prompt管理与评测规格.md`

---

## 目录

1. [LLMOps观测体系设计](#第1章-llmops观测体系设计)
2. [请求Trace数据模型](#第2章-请求trace数据模型)
3. [Session会话视图](#第3章-session会话视图)
4. [成本归因分析](#第4章-成本归因分析)
5. [LLMOps仪表板规格](#第5章-llmops仪表板规格)
6. [异常检测与聚类](#第6章-异常检测与聚类)
7. [Trace审计能力](#第7章-trace审计能力)
8. [Trace数据保留策略](#第8章-trace数据保留策略)
9. [告警规则配置](#第9章-告警规则配置)
10. [Trace API与SDK集成](#第10章-trace-api与sdk集成)
11. [Agent Trace 专节](#第11章-agent-trace-专节)
12. [验收标准](#第12章-验收标准)

---

## 第1章 LLMOps观测体系设计

### 1.1 为什么需要专属的LLMOps可观测性

传统软件工程的可观测性（Observability）围绕"延迟—吞吐—错误"三要素（RED方法论）构建，APM工具如SkyWalking、Jaeger、Datadog在微服务链路追踪领域已高度成熟。但大模型请求具有一系列传统APM完全无法覆盖的独特特征：

**Token经济性**：LLM请求的"成本"不是计算资源消耗，而是Token消费量乘以单价，同一个API端点的请求成本差异可达1000倍以上（取决于输入输出长度）。传统APM只能追踪接口调用次数和耗时，无法建立Token级别的计量和成本归因。

**首Token延迟（TTFB）的特殊性**：用户体验取决于流式响应下第一个Token何时到达，而非整个请求何时完成。P95 TTFB是评价LLM服务质量最关键的单一指标，传统APM对流式响应的延迟分解缺乏原生支持。

**多跳路由的透明性**：MaaS平台的每次请求背后可能经历：缓存查询 → 合规检查 → 路由策略评估 → 候选模型筛选 → 主后端调用 → fallback触发 → 计费扣款 等多个内部阶段，每个阶段都有独立的耗时、状态和可能的失败点。业务方需要知道"这次请求为什么用了3.2秒"的详细答案，传统链路追踪的span树可以显示物理调用链，但无法呈现路由决策的"理由"。

**模型质量维度**：除了延迟和成功率，LLM请求还有内容安全评分、输出质量评估、Prompt与Response的语义相关性等质量维度，这些完全超出传统APM的监控范畴。

**Session上下文关联**：多轮对话的每一轮在HTTP层面是独立请求，但业务层面需要关联为一个Session进行整体分析（累计成本、对话轮数、上下文长度演进）。传统APM的Trace只能关联单次请求，无法建立跨请求的对话视图。

**fallback链的因果追溯**：当主模型不可用、fallback到备用模型时，用户感知到的是一次成功响应，但背后可能是"主模型超时 → 第一备用模型限流 → 第二备用模型成功响应"的三跳链路。需要记录完整的fallback决策链，便于事后分析。

正是因为上述六个维度的特殊性，MaaS平台必须构建一套专为LLM请求设计的可观测性体系，简称LLMOps观测体系。

### 1.2 LLMOps观测的四个层次

```
┌─────────────────────────────────────────────────────────────────┐
│  Business 层（业务洞察）                                          │
│  成本趋势 / ROI分析 / 模型采用率 / 供应商依赖度                    │
├─────────────────────────────────────────────────────────────────┤
│  Session 层（会话视图）                                           │
│  多轮对话关联 / Session成本 / 上下文长度演进 / 异常Session识别      │
├─────────────────────────────────────────────────────────────────┤
│  Trace 层（请求级追踪）                                           │
│  单次请求全链路 / 路由解释 / fallback链 / Span耗时分解              │
├─────────────────────────────────────────────────────────────────┤
│  Metrics 层（聚合指标）                                           │
│  请求量 / 成功率 / P95延迟 / TTFB / 成本 / 缓存命中率              │
└─────────────────────────────────────────────────────────────────┘
```

四个层次从下到上，数据粒度从聚合到明细，查询成本从低到高，存储体量从小到大。平台需要四个层次协同工作，满足不同角色的可观测需求：

| 角色 | 主要使用层次 | 典型需求 |
|------|------------|---------|
| 企业管理员 | Business + Metrics | 每月模型成本趋势、供应商集中度风险 |
| 项目负责人 | Metrics + Session | 项目API成本、哪些Session异常消耗高 |
| 开发工程师 | Trace + Metrics | 调试特定请求为什么慢、为什么触发fallback |
| 财务审计员 | Trace（审计级）| 对账特定时段的计费数据 |
| 安全合规员 | Trace（合规级）| 审查内容安全检测记录、合规策略执行证明 |
| 平台运维 | Metrics + 告警 | 全局异常检测、供应商健康告警 |

### 1.3 数据采集链路架构

```mermaid
flowchart TD
    A[客户端请求\nHTTP/SSE] --> B[Gateway Service\n请求接入 & 鉴权]
    B --> C{缓存查询\nRedis}
    C -->|命中| D[Cache Hit\n直接返回]
    C -->|未命中| E[合规检查\nCompliance Service]
    E --> F[路由策略引擎\nRouting Service]
    F --> G[候选模型评分\nScoring Engine]
    G --> H[选定主后端\nVendor Adapter]
    H -->|成功| I[响应流转\nSSE / 同步]
    H -->|失败/超时| J[Fallback触发\nFallback Chain]
    J --> K[备用后端\nVendor Adapter N]
    K --> I
    I --> L[计费扣款\nBilling Service]
    L --> M[Trace写入\nTrace Collector]

    subgraph 观测数据流
        M --> N[Kafka 消息队列]
        N --> O[实时流处理\nFlink/Storm]
        N --> P[Trace存储\nClickHouse]
        O --> Q[实时指标\nInfluxDB/VictoriaMetrics]
        O --> R[异常检测引擎]
        P --> S[Trace查询API]
        Q --> T[仪表板\nGrafana/自研]
        R --> U[告警通知]
    end

    subgraph 冷存储
        P -->|TTL>30天| V[对象存储\nOSS/S3]
        V --> W[审计查询\n按需加载]
    end
```

### 1.4 存储策略与成本控制

LLMOps观测数据的体量与业务规模线性增长。对于每日处理100万次请求的平台，若完整存储每条Trace（含Prompt和Response内容），每天的存储增量约为10~50GB。按180天保留期，累计存储量在1.8TB~9TB之间。必须通过分层存储策略控制存储成本。

**热数据（0~7天）**：完整Trace，包含所有字段、trace_spans详情、route_explanation对象。存储在ClickHouse的高频查询表，支持按任意字段的即时查询，P95查询延迟 < 2秒。热数据保留全量Prompt/Response内容（除非租户开启零数据保留模式）。

**温数据（8~30天）**：Trace元数据 + 聚合统计。仅保留trace_id、核心维度字段（tenant_id/project_id/model_id/vendor/status/token数/成本）以及按小时粒度的聚合指标，不保留trace_spans详情和完整Prompt/Response。查询延迟允许 < 10秒。

**冷数据（31~180天）**：仅审计相关字段。保留trace_id、计费关键字段（input_tokens/output_tokens/total_cost/billing_snapshot）、合规策略执行记录（compliance_policy_id/content_safety_result）。压缩存储至对象存储（OSS/S3），按需加载，查询延迟允许 < 60秒。

**零数据保留模式**：租户可配置此模式，平台不存储任何Prompt和Response内容。仅记录Token数量和成本元数据。此模式适合金融、政务等对数据主权高度敏感的客户。

| 数据分层 | 保留时长 | 存储介质 | 查询延迟 | 包含内容 |
|--------|--------|--------|--------|--------|
| 热数据 | 0~7天 | ClickHouse 本地磁盘 | < 2s | 全量Trace含内容 |
| 温数据 | 8~30天 | ClickHouse 分层存储 | < 10s | 元数据 + 聚合统计 |
| 冷数据 | 31~180天 | 对象存储OSS/S3 | < 60s | 审计字段子集 |
| 归档 | 180天+ | 归档存储（低频） | 分钟级 | 审计最小集合 |

---

## 第2章 请求Trace数据模型

### 2.1 设计原则

Trace数据模型是整个LLMOps观测体系的基础。设计时遵循以下原则：

**完备性优先**：宁可多记录字段，也不在事后发现关键信息缺失。字段增加的存储成本远小于问题排查失败的业务成本。

**不可变性**：Trace一旦写入，除append操作（如更新completed_at和output相关字段）外，不允许修改已有字段值，确保审计可信度。

**自描述性**：route_explanation字段包含完整的路由决策上下文，即使策略配置后续变更，历史Trace依然能够自解释当时的路由逻辑。

**查询友好**：高频查询维度（tenant_id、project_id、model_id、status、created_at）作为独立字段存储，不嵌套在JSON中，确保查询性能。

### 2.2 顶层Trace对象（trace表）

以下是trace表的完整字段定义，共44个字段：

| 字段名 | 类型 | 可空 | 说明 |
|-------|------|------|------|
| trace_id | UUID | N | 全局唯一Trace标识，由Gateway Service生成，格式：`trc_` + UUIDv7 |
| request_id | VARCHAR(64) | N | 客户端传入的请求ID（x-request-id头），若客户端未传则等于trace_id |
| session_id | VARCHAR(64) | Y | 会话ID，客户端传入（x-session-id头）或由平台根据历史messages自动关联 |
| tenant_id | VARCHAR(32) | N | 租户ID，从API Key解析 |
| project_id | VARCHAR(32) | N | 项目ID，从API Key解析 |
| api_key_id | VARCHAR(32) | N | 调用所用API Key的ID（非Key明文），用于成本归因和权限审计 |
| user_id | VARCHAR(256) | Y | 请求方终端用户标识，从请求body中的`user`字段或自定义header取，不经平台校验 |
| model_id | VARCHAR(64) | N | 逻辑模型ID，客户端在请求中指定的model参数，如`gpt-4o`、`qwen-turbo` |
| model_alias | VARCHAR(64) | Y | 逻辑模型别名，若客户端使用别名访问时记录原始alias |
| vendor_backend_id | VARCHAR(64) | N | 实际路由到的后端标识，如`openai-us-east-pool-1`、`aliyun-dashscope-main` |
| vendor_name | VARCHAR(32) | N | 供应商名称，冗余存储便于查询，如`openai`、`aliyun`、`anthropic` |
| vendor_model_name | VARCHAR(64) | N | 实际调用的供应商侧模型名称，如`gpt-4o-2024-11-20` |
| policy_id | VARCHAR(32) | Y | 路由策略ID，NULL表示使用默认策略 |
| policy_version | INT | Y | 路由策略版本号，确保策略变更后历史Trace仍可对应当时生效的策略版本 |
| request_started_at | TIMESTAMP(3) | N | 请求进入Gateway的时间戳，毫秒精度 |
| first_token_at | TIMESTAMP(3) | Y | 收到第一个响应Token的时间戳，流式请求有效，同步请求为NULL |
| completed_at | TIMESTAMP(3) | Y | 请求完成时间戳，含最后一个Token到达时间 |
| ttfb_ms | INT | Y | 首Token延迟（Time To First Byte），毫秒，`= first_token_at - request_started_at` |
| total_latency_ms | INT | Y | 请求总延迟，毫秒，`= completed_at - request_started_at` |
| vendor_latency_ms | INT | Y | 供应商侧延迟，毫秒，排除MaaS平台内部处理耗时 |
| platform_overhead_ms | INT | Y | 平台内部处理耗时，毫秒，`= total_latency_ms - vendor_latency_ms` |
| input_tokens | INT | N | 输入Token数量，由供应商返回或平台估算 |
| output_tokens | INT | Y | 输出Token数量，由供应商返回；若请求失败则为0 |
| total_tokens | INT | N | 总Token数量，`= input_tokens + output_tokens` |
| input_cost_cny | DECIMAL(12,6) | N | 输入Token费用，人民币，按当时生效的定价计算 |
| output_cost_cny | DECIMAL(12,6) | N | 输出Token费用，人民币 |
| total_cost_cny | DECIMAL(12,6) | N | 本次请求总费用，人民币，`= input_cost_cny + output_cost_cny` |
| pricing_snapshot | JSONB | N | 计费时生效的定价快照，防止事后定价变更导致成本无法复核 |
| cache_hit | BOOLEAN | N | 是否命中语义缓存，默认false |
| cache_key | VARCHAR(64) | Y | 命中的缓存Key的哈希值，cache_hit=true时有值 |
| cache_saved_tokens | INT | Y | 因缓存命中节省的Token数（估算），cache_hit=true时有值 |
| cache_saved_cost_cny | DECIMAL(12,6) | Y | 因缓存命中节省的费用，人民币 |
| status | VARCHAR(32) | N | 请求结果状态：`success` / `error` / `timeout` / `content_blocked` / `fallback_success` / `rate_limited` / `quota_exceeded` |
| http_status_code | SMALLINT | N | 向客户端返回的HTTP状态码 |
| error_code | VARCHAR(64) | Y | 平台错误码，如`VENDOR_TIMEOUT`、`CONTENT_POLICY_VIOLATION`、`QUOTA_EXCEEDED` |
| error_message | TEXT | Y | 错误信息，不含敏感内容 |
| fallback_triggered | BOOLEAN | N | 是否触发了fallback，默认false |
| fallback_count | SMALLINT | N | fallback触发次数，0表示未触发 |
| compliance_policy_id | VARCHAR(32) | Y | 命中并执行的合规策略ID，NULL表示未命中任何合规策略 |
| content_safety_result | JSONB | Y | 内容安全检测结果，包含检测器名称、风险等级、拦截原因 |
| request_metadata | JSONB | Y | 客户端传入的业务标签，通过x-trace-tags头传递，用于自定义成本归因维度，如`{"env":"prod","feature":"chat","version":"v2"}` |
| trace_spans | JSONB | N | Trace Span数组，记录请求内各阶段详情，见2.3节 |
| route_explanation | JSONB | Y | 路由解释对象，记录路由决策过程，见2.4节 |

### 2.3 Trace Span子对象

trace_spans是一个JSON数组，每个元素代表请求处理流程中的一个阶段。Span的设计参考OpenTelemetry规范，但针对LLM请求进行了扩展。

**Span类型定义：**

| span_type | 说明 | 典型duration |
|-----------|------|------------|
| `auth_check` | 鉴权与API Key校验 | < 5ms |
| `quota_check` | 配额检查 | < 10ms |
| `cache_check` | 语义缓存查询 | 5~50ms |
| `compliance_check` | 合规策略执行（Prompt检查） | 10~200ms |
| `route_eval` | 路由策略评估与模型选择 | 5~30ms |
| `vendor_call` | 供应商API调用（含网络往返）| 500ms~30s |
| `stream_relay` | 流式响应转发 | 与vendor_call重叠 |
| `fallback` | Fallback触发与切换 | < 5ms（切换决策本身）|
| `output_compliance` | 响应内容安全检查 | 10~500ms |
| `billing` | 计费记账 | < 20ms |

**Span字段定义：**

| 字段名 | 类型 | 可空 | 说明 |
|-------|------|------|------|
| span_id | VARCHAR(32) | N | Span唯一标识，在同一Trace内唯一 |
| trace_id | UUID | N | 所属Trace的ID |
| parent_span_id | VARCHAR(32) | Y | 父Span ID，顶层Span为NULL |
| span_type | VARCHAR(32) | N | Span类型，见上表 |
| span_name | VARCHAR(128) | N | Span名称，人类可读，如`cache_check:redis_lookup` |
| started_at | TIMESTAMP(3) | N | Span开始时间 |
| ended_at | TIMESTAMP(3) | Y | Span结束时间，进行中的Span为NULL |
| duration_ms | INT | Y | Span耗时，毫秒 |
| span_status | VARCHAR(16) | N | `ok` / `error` / `timeout` / `skipped` |
| span_detail | JSONB | Y | Span详情，各类型有不同的detail结构 |

**vendor_call类型的span_detail结构示例：**

```json
{
  "vendor_backend_id": "openai-us-east-pool-1",
  "vendor_model": "gpt-4o-2024-11-20",
  "endpoint_url": "https://api.openai.com/v1/chat/completions",
  "http_status": 200,
  "attempt_number": 1,
  "retry_count": 0,
  "is_streaming": true,
  "first_token_ms": 312,
  "tokens_per_second": 45.2,
  "total_vendor_latency_ms": 2840
}
```

**fallback类型的span_detail结构示例：**

```json
{
  "fallback_reason": "VENDOR_TIMEOUT",
  "from_backend": "openai-us-east-pool-1",
  "to_backend": "anthropic-claude-3-5-sonnet",
  "fallback_position": 1,
  "timeout_threshold_ms": 5000,
  "actual_wait_ms": 5023
}
```

### 2.4 Route Explanation对象

route_explanation记录路由引擎选择模型的完整决策过程，是平台"可解释路由"能力的核心数据载体。

```json
{
  "policy_id": "pol_abc123",
  "policy_name": "生产环境-质量优先策略",
  "policy_version": 7,
  "evaluation_time_ms": 12,
  "candidate_models": [
    {
      "model_id": "gpt-4o",
      "vendor_backend_id": "openai-us-east-pool-1",
      "total_score": 0.87,
      "score_breakdown": {
        "health_score": 0.95,
        "latency_score": 0.82,
        "cost_score": 0.71,
        "quality_score": 0.90,
        "priority_bonus": 0.10
      },
      "was_selected": true
    },
    {
      "model_id": "claude-3-5-sonnet",
      "vendor_backend_id": "anthropic-main-pool",
      "total_score": 0.81,
      "score_breakdown": {
        "health_score": 0.98,
        "latency_score": 0.78,
        "cost_score": 0.85,
        "quality_score": 0.88,
        "priority_bonus": 0.00
      },
      "was_selected": false
    }
  ],
  "excluded_models": [
    {
      "model_id": "gpt-4-turbo",
      "vendor_backend_id": "openai-us-east-pool-1",
      "exclusion_reason": "QUOTA_EXHAUSTED",
      "exclusion_detail": "项目级配额已耗尽，剩余0 CNY"
    }
  ],
  "selected_model": {
    "model_id": "gpt-4o",
    "vendor_backend_id": "openai-us-east-pool-1",
    "selection_reason": "综合评分最高（0.87），满足质量优先策略的质量权重要求"
  },
  "fallback_chain": [
    {"position": 0, "model_id": "gpt-4o", "vendor_backend_id": "openai-us-east-pool-1"},
    {"position": 1, "model_id": "claude-3-5-sonnet", "vendor_backend_id": "anthropic-main-pool"},
    {"position": 2, "model_id": "qwen-max", "vendor_backend_id": "aliyun-dashscope-main"}
  ]
}
```

### 2.5 完整建表DDL

```sql
-- 主Trace表（ClickHouse）
CREATE TABLE maas.traces (
    trace_id            UUID           NOT NULL,
    request_id          String         NOT NULL,
    session_id          String         DEFAULT '',
    tenant_id           String         NOT NULL,
    project_id          String         NOT NULL,
    api_key_id          String         NOT NULL,
    user_id             String         DEFAULT '',
    model_id            String         NOT NULL,
    model_alias         String         DEFAULT '',
    vendor_backend_id   String         NOT NULL,
    vendor_name         String         NOT NULL,
    vendor_model_name   String         NOT NULL,
    policy_id           String         DEFAULT '',
    policy_version      Int32          DEFAULT 0,
    request_started_at  DateTime64(3)  NOT NULL,
    first_token_at      DateTime64(3)  DEFAULT '1970-01-01',
    completed_at        DateTime64(3)  DEFAULT '1970-01-01',
    ttfb_ms             Int32          DEFAULT 0,
    total_latency_ms    Int32          DEFAULT 0,
    vendor_latency_ms   Int32          DEFAULT 0,
    platform_overhead_ms Int32         DEFAULT 0,
    input_tokens        Int32          NOT NULL,
    output_tokens       Int32          DEFAULT 0,
    total_tokens        Int32          NOT NULL,
    input_cost_cny      Decimal(12,6)  NOT NULL,
    output_cost_cny     Decimal(12,6)  NOT NULL,
    total_cost_cny      Decimal(12,6)  NOT NULL,
    pricing_snapshot    String         DEFAULT '{}',
    cache_hit           UInt8          DEFAULT 0,
    cache_key           String         DEFAULT '',
    cache_saved_tokens  Int32          DEFAULT 0,
    cache_saved_cost_cny Decimal(12,6) DEFAULT 0,
    status              String         NOT NULL,
    http_status_code    Int16          NOT NULL,
    error_code          String         DEFAULT '',
    error_message       String         DEFAULT '',
    fallback_triggered  UInt8          DEFAULT 0,
    fallback_count      Int16          DEFAULT 0,
    compliance_policy_id String        DEFAULT '',
    content_safety_result String       DEFAULT '{}',
    request_metadata    String         DEFAULT '{}',
    trace_spans         String         DEFAULT '[]',
    route_explanation   String         DEFAULT '{}'
)
ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(request_started_at)
ORDER BY (tenant_id, project_id, request_started_at, trace_id)
TTL request_started_at + INTERVAL 180 DAY
SETTINGS index_granularity = 8192;

-- 低基数字段物化索引，加速过滤查询
ALTER TABLE maas.traces ADD INDEX idx_status status TYPE set(10) GRANULARITY 1;
ALTER TABLE maas.traces ADD INDEX idx_vendor vendor_name TYPE set(20) GRANULARITY 1;
ALTER TABLE maas.traces ADD INDEX idx_model model_id TYPE set(100) GRANULARITY 1;
ALTER TABLE maas.traces ADD INDEX idx_session session_id TYPE bloom_filter(0.01) GRANULARITY 1;
```

---

## 第3章 Session会话视图

### 3.1 Session的定义与识别规则

在多轮对话场景中，用户与AI模型的交互由多次HTTP请求组成，每次请求在API层面完全独立。Session（会话）是将这些逻辑上连续的请求关联为一个整体分析单元的抽象。

**Session识别方式（优先级从高到低）：**

1. **显式Session ID**：客户端在请求header中传入`x-session-id`，平台信任此值，将请求归属到对应Session。这是推荐方式，适用于客户端有会话管理能力的场景。

2. **messages历史关联**：对于OpenAI Chat Completions格式的请求，客户端通常会在`messages`数组中携带完整对话历史。平台对messages数组（不含最后一条用户消息）计算哈希，若与已有Session的最后一轮messages哈希匹配，则将该请求关联到已有Session，并分配新的Session轮次。

3. **API Key + 时间窗口**：若前两种方式均无法识别，且同一API Key在5分钟内发出的连续请求，可选择性地聚合到同一推断Session（默认关闭，需租户显式开启此推断模式）。

4. **独立请求**：无法关联Session的请求，session_id字段为空，在Session视图中不显示。

### 3.2 session表字段定义

| 字段名 | 类型 | 可空 | 说明 |
|-------|------|------|------|
| session_id | VARCHAR(64) | N | 会话唯一标识 |
| tenant_id | VARCHAR(32) | N | 所属租户ID |
| project_id | VARCHAR(32) | N | 所属项目ID |
| api_key_id | VARCHAR(32) | N | 发起会话的API Key ID |
| user_id | VARCHAR(256) | Y | 终端用户标识（来自请求方传入） |
| model_id | VARCHAR(64) | N | 会话主要使用的逻辑模型ID，取turn_count最多的模型 |
| model_history | JSONB | N | 会话中使用过的所有模型及各模型轮次数，如`[{"model":"gpt-4o","turns":5},{"model":"claude-3-5","turns":2}]` |
| turn_count | INT | N | 对话轮数（每次请求算一轮） |
| total_input_tokens | BIGINT | N | 会话累计输入Token数 |
| total_output_tokens | BIGINT | N | 会话累计输出Token数 |
| total_tokens | BIGINT | N | 会话累计总Token数 |
| total_cost_cny | DECIMAL(14,6) | N | 会话累计总费用，人民币 |
| avg_ttfb_ms | INT | Y | 会话内各轮TTFB的平均值 |
| p95_ttfb_ms | INT | Y | 会话内各轮TTFB的P95值 |
| avg_latency_ms | INT | Y | 会话内各轮总延迟的平均值 |
| created_at | TIMESTAMP(3) | N | 会话第一次请求的时间 |
| last_active_at | TIMESTAMP(3) | N | 会话最后一次请求的时间 |
| session_duration_s | INT | N | 会话持续时长，秒，`= last_active_at - created_at` |
| success_count | INT | N | 成功请求数 |
| error_count | INT | N | 错误请求数 |
| fallback_count | INT | N | fallback触发次数 |
| cache_hit_count | INT | N | 缓存命中次数 |
| context_length_tokens | INT | Y | 当前会话的上下文长度估算（累计messages的Token数） |
| max_context_tokens | INT | Y | 会话中出现过的最大上下文长度 |
| is_anomalous | BOOLEAN | N | 是否被标记为异常Session（如成本异常、循环调用等） |
| anomaly_reasons | JSONB | Y | 异常原因列表，is_anomalous=true时有值 |
| compliance_flags | JSONB | Y | 会话级合规标记，如内容安全拦截记录 |
| metadata | JSONB | Y | 从各轮请求request_metadata合并的业务标签 |

### 3.3 Session视图功能

**多轮对话时间线**：以时间轴形式展示Session内每一轮请求，显示各轮的TTFB、总延迟、Token数、费用、状态。支持展开单轮查看完整Trace详情。

**累计成本趋势**：折线图显示会话内累计成本随轮次的增长曲线，帮助识别哪些轮次消耗异常。

**上下文长度演进**：折线图显示随轮次增加，messages数组的Token长度变化，辅助判断是否存在不必要的长上下文。

**模型切换记录**：当Session内发生模型切换（fallback或主动切换），以不同颜色高亮显示切换事件，并注明切换原因。

**Session聚合查询**示例（ClickHouse SQL）：

```sql
-- 按项目统计过去24小时的Session指标
SELECT
    project_id,
    count()                           AS session_count,
    avg(turn_count)                   AS avg_turns,
    avg(total_cost_cny)               AS avg_session_cost,
    quantile(0.95)(total_cost_cny)    AS p95_session_cost,
    sum(is_anomalous)                 AS anomalous_session_count,
    avg(session_duration_s)           AS avg_duration_s
FROM maas.sessions
WHERE tenant_id = 'tenant_abc'
  AND created_at >= now() - INTERVAL 24 HOUR
GROUP BY project_id
ORDER BY avg_session_cost DESC;
```

### 3.4 超长Session识别与分析

超长Session是成本失控的重要来源。以下规则用于识别超长Session并触发告警：

| 规则 | 触发条件 | 风险等级 | 建议操作 |
|------|---------|--------|---------|
| 高成本Session | 单Session总费用 > 租户P99会话成本的3倍 | P1 | 通知项目负责人审查 |
| 极长上下文 | context_length_tokens > 模型上下文窗口的80% | P2 | 建议客户端实现上下文压缩 |
| 高频循环调用 | turn_count > 100且session_duration_s < 300 | P1 | 疑似程序性循环调用，触发限流检查 |
| 长时间活跃 | session_duration_s > 86400（超过24小时）| P3 | 记录但不告警，可能是有效长程交互 |
| 频繁fallback | fallback_count / turn_count > 0.3 | P2 | 供应商质量问题或路由策略需调整 |

---

## 第4章 成本归因分析

### 4.1 成本归因维度体系

成本归因是企业客户使用MaaS平台的核心诉求之一。企业财务团队需要知道"AI成本花在哪里了"，技术团队需要知道"哪个项目、哪个功能消耗了最多Token"。MaaS平台的成本归因体系支持8个归因维度的任意组合：

| 归因维度 | 数据来源 | 典型用途 |
|--------|---------|---------|
| 租户（Tenant） | trace.tenant_id | 多租户平台的成本分摊 |
| 项目（Project） | trace.project_id | 项目级成本预算管理 |
| API Key | trace.api_key_id | 追踪特定服务/应用的成本 |
| 逻辑模型（Model） | trace.model_id | 了解各模型的成本占比 |
| 供应商（Vendor） | trace.vendor_name | 供应商采购谈判依据 |
| 时段（Time） | trace.request_started_at | 识别成本高峰时段 |
| 终端用户（User） | trace.user_id | 用户级成本追踪（如按用户收费的产品） |
| 业务标签（Tag） | trace.request_metadata | 自定义成本中心，如按功能模块/业务线/环境 |

### 4.2 成本归因数据聚合架构

```mermaid
sequenceDiagram
    participant GW as Gateway Service
    participant Kafka as Kafka消息队列
    participant Flink as Flink实时流处理
    participant Redis as Redis计数器
    participant CH as ClickHouse
    participant Budget as 预算服务
    participant Alert as 告警服务

    GW->>Kafka: 发布 trace_completed 事件（含完整计费信息）
    Kafka->>Flink: 消费事件（延迟 < 500ms）
    
    par 实时路径
        Flink->>Redis: INCRBY 项目实时成本计数器
        Flink->>Redis: INCRBY 租户实时Token计数器
        Redis->>Budget: 触发预算检查（每次写入后）
        Budget-->>Alert: 预算阈值触发告警
    and 写入ClickHouse
        Flink->>CH: 写入 cost_hourly_agg 聚合表（小时粒度）
        Flink->>CH: 写入 traces 明细表
    end
    
    Note over Flink,CH: 离线批处理（每小时）
    CH->>CH: 物化视图 cost_daily_agg 增量聚合
    CH->>CH: 物化视图 cost_monthly_agg 增量聚合
```

### 4.3 成本归因报表字段设计

**实时成本聚合表（cost_realtime_cache，Redis）：**

使用Redis Hash结构存储实时成本计数，Key格式：`maas:cost:{tenant_id}:{project_id}:{date}:{model_id}`

```
字段：
  input_tokens_total    : 累计输入Token（BIGINT）
  output_tokens_total   : 累计输出Token（BIGINT）
  cost_fen              : 累计费用（分，整数避免浮点精度问题）
  request_count         : 请求总数
  error_count           : 错误请求数
  last_updated_at       : 最后更新时间戳（毫秒）
```

**小时成本聚合表（cost_hourly_agg，ClickHouse）：**

| 字段名 | 类型 | 说明 |
|-------|------|------|
| agg_hour | DateTime | 聚合小时（如`2026-05-21 14:00:00`） |
| tenant_id | String | 租户ID |
| project_id | String | 项目ID |
| api_key_id | String | API Key ID |
| model_id | String | 逻辑模型ID |
| vendor_name | String | 供应商名称 |
| request_count | UInt64 | 请求数 |
| success_count | UInt64 | 成功请求数 |
| error_count | UInt64 | 错误请求数 |
| input_tokens | UInt64 | 输入Token总数 |
| output_tokens | UInt64 | 输出Token总数 |
| total_cost_cny | Decimal(14,6) | 总费用 |
| cache_hit_count | UInt64 | 缓存命中次数 |
| cache_saved_cost_cny | Decimal(14,6) | 缓存节省费用 |
| fallback_count | UInt64 | fallback次数 |
| avg_ttfb_ms | Float32 | 平均TTFB |
| p95_ttfb_ms | Float32 | P95 TTFB |
| p99_latency_ms | Float32 | P99 总延迟 |

### 4.4 预算消耗实时同步机制

平台对预算的消耗追踪需要做到准实时（延迟 < 2秒），避免因延迟导致预算超支。实现机制如下：

**三层预算保护：**

- **硬限额（Hard Limit）**：通过Redis原子计数器实现，每次请求完成后INCRBY对应计数器，在下次请求时PRE-CHECK（非请求后扣款），确保超出硬限额的请求被拦截。
- **软限额（Soft Limit）**：达到软限额时发送告警但不拦截请求，让管理员有预警时间调整预算。
- **预测告警（Forecast Alert）**：基于当前消耗速率，预测若按此速率到月末时是否会超出预算，提前N天告警。

### 4.5 成本异常检测规则

| 规则名称 | 检测方法 | 触发条件 | 风险等级 |
|--------|---------|---------|--------|
| Token暴涨检测 | 与过去7天同时段均值对比 | 当前小时Token数 > 均值×3 | P1 |
| 单请求超大检测 | 绝对阈值 | 单次请求total_tokens > 100,000 | P2 |
| 成本速率异常 | 滑动窗口 | 最近5分钟成本速率 > 历史P99×5 | P0 |
| 模型价格变动提醒 | 价格表变更触发 | 供应商价格变动 > ±10% | P3 |
| 负毛利预警 | 转售价 vs 采购价 | 转售价 < 采购价（存在折扣时） | P1 |
| 零成本高Token | 逻辑检测 | token > 0 但 cost = 0 | P0 |
| 缓存节省下滑 | 趋势检测 | 缓存命中率环比下降 > 30% | P3 |

---

## 第5章 LLMOps仪表板规格

### 5.1 实时概览看板

实时概览看板面向平台运维和项目负责人，展示当前15分钟窗口的关键指标，自动刷新间隔30秒。

**KPI指标卡定义（共8个核心指标）：**

| 指标名称 | 指标定义 | 计算口径 | 参考阈值（正常/警告/异常）|
|--------|---------|---------|----------------------|
| 请求量（RPM） | 最近1分钟请求总数 | COUNT(trace_id) WHERE request_started_at >= now()-60s | - / - / - （无绝对阈值，看环比）|
| 成功率 | 成功请求占比 | COUNT(status='success') / COUNT(*) × 100% | > 99% / 95~99% / < 95% |
| P95 TTFB | 首Token延迟P95 | QUANTILE(0.95)(ttfb_ms) WHERE status='success' | < 1000ms / 1~3s / > 3s |
| P99 总延迟 | 请求总延迟P99 | QUANTILE(0.99)(total_latency_ms) | < 5000ms / 5~15s / > 15s |
| 每分钟成本 | 最近1分钟费用总和 | SUM(total_cost_cny) WHERE request_started_at >= now()-60s | 预算分摊值×1.2 触发告警 |
| 缓存命中率 | 缓存命中请求占比 | COUNT(cache_hit=1) / COUNT(*) × 100% | > 20% / 10~20% / < 10% |
| Fallback率 | Fallback触发请求占比 | COUNT(fallback_triggered=1) / COUNT(*) × 100% | < 1% / 1~5% / > 5% |
| 错误率 | 非成功请求占比 | COUNT(status!='success') / COUNT(*) × 100% | < 1% / 1~5% / > 5% |

### 5.2 模型性能趋势

折线图展示过去24小时各模型的延迟趋势对比，支持以下配置：

- **时间粒度**：5分钟/15分钟/1小时可切换
- **指标选择**：P50延迟 / P95延迟 / P99延迟 / TTFB
- **模型筛选**：支持多选，最多同时展示8条折线
- **对比参考线**：可叠加显示SLA目标线（如P95 < 2000ms）

**告警联动**：当某模型的趋势线超过SLA目标线持续5分钟，触发P2告警。

### 5.3 成本分布（Treemap）

以Treemap可视化展示成本在各维度上的分布，支持三级钻取：

- **第一级**：各项目的成本占比
- **第二级（点击项目展开）**：该项目内各逻辑模型的成本占比
- **第三级（点击模型展开）**：该模型在各供应商后端的成本分布

Treemap中每个矩形颜色深浅表示成本增长速率（深色=增长快）。

### 5.4 错误分析

**按错误码分类的柱状图，展示最近24小时内各错误类型的发生次数：**

| 错误码类别 | 错误码列表 |
|----------|---------|
| 供应商错误 | VENDOR_TIMEOUT / VENDOR_RATE_LIMIT / VENDOR_API_ERROR / VENDOR_OVERLOAD |
| 平台错误 | QUOTA_EXCEEDED / BUDGET_EXCEEDED / RATE_LIMITED / POLICY_VIOLATION |
| 内容错误 | CONTENT_POLICY_VIOLATION / INPUT_TOO_LONG / INVALID_REQUEST |
| 网络错误 | NETWORK_TIMEOUT / CONNECTION_REFUSED / DNS_FAILURE |

支持点击错误码钻取到对应的Trace列表，直接进行根因排查。

### 5.5 路由分布（Sankey图）

Sankey图展示请求在路由层的流向分布：

```
请求量总数
    ├── 逻辑模型A（如gpt-4o）
    │       ├── 供应商后端A1（如openai-us-east）
    │       └── 供应商后端A2（如openai-eu-west，fallback）
    ├── 逻辑模型B（如claude-3-5-sonnet）
    │       └── 供应商后端B1（如anthropic-main）
    └── 缓存命中（直接返回，不到达供应商）
```

Sankey图的节点宽度与请求量成比例，颜色区分状态（绿色=成功，红色=有错误）。

### 5.6 缓存效果分析

缓存效果面板展示以下指标（时间范围可选：今日/7日/30日）：

| 指标 | 说明 | 计算口径 |
|------|------|---------|
| 缓存命中率（%） | 命中缓存的请求占比 | cache_hit_count / total_requests |
| 节省Token数 | 因命中缓存节省的Token数量 | SUM(cache_saved_tokens) |
| 节省金额（CNY）| 因命中缓存节省的费用 | SUM(cache_saved_cost_cny) |
| 节省比例（%）| 节省金额 / (实际成本 + 节省金额) | 节省成本占理论总成本的比例 |
| 平均缓存延迟 | 命中缓存时的平均响应延迟 | AVG(total_latency_ms WHERE cache_hit=1) |

---

## 第6章 异常检测与聚类

### 6.1 异常类型分类

MaaS平台的LLM请求异常可分为五大类，每类有不同的检测算法和处理策略：

**性能异常**：请求延迟、TTFB显著高于历史基线。典型场景：供应商机房故障导致延迟升高、某模型资源紧张导致吞吐下降。

**成本异常**：Token消耗或费用显著高于预期。典型场景：客户端bug导致大量重复请求、无限循环调用、超长Prompt传入。

**质量异常**：成功率下降、错误率上升。典型场景：供应商API版本更新导致兼容性问题、网络抖动导致超时率升高。

**合规异常**：内容安全检测触发率异常升高，或特定合规策略频繁命中。典型场景：用户尝试注入攻击、内容政策配置错误导致正常请求被拦截。

**供应商异常**：特定供应商的错误率、延迟指标明显劣化。触发供应商健康评分降级，联动路由策略调整流量。

### 6.2 异常检测算法

**3σ规则（正态分布假设）：**

对于延迟、Token数等近似正态分布的指标，使用3σ规则检测点异常：若当前值超过均值±3倍标准差，则判定为异常。适用于历史数据量充足（> 1000样本）的场景。

```
异常阈值上界 = μ + 3σ
异常阈值下界 = μ - 3σ
其中 μ 和 σ 基于过去7天同时段（±1小时窗口）的历史数据计算
```

**指数移动平均（EMA）：**

对于成本速率、请求量等时序指标，使用EMA检测趋势异常，平滑短期波动：

```
EMA(t) = α × X(t) + (1-α) × EMA(t-1)
α = 2/(N+1)，N为平滑周期（默认N=12，即12个5分钟窗口 = 1小时）
异常触发条件：X(t) > EMA(t-1) × (1 + threshold)
threshold 根据指标类型配置（成本速率默认3.0，即超出EMA 300%）
```

**历史同期对比（同比/环比）：**

对于日/周周期性明显的指标（如工作日请求量），使用同比对比：

```
异常判断：当前小时指标 > 过去4周同一小时均值 × (1 + 允许偏差)
允许偏差默认配置：请求量=0.5（+50%），成本=0.3（+30%），延迟=0.5（+50%）
```

### 6.3 异常事件数据模型

| 字段名 | 类型 | 可空 | 说明 |
|-------|------|------|------|
| anomaly_id | UUID | N | 异常事件唯一ID |
| anomaly_type | VARCHAR(32) | N | 异常类型：`performance` / `cost` / `quality` / `compliance` / `vendor` |
| severity | VARCHAR(8) | N | 严重程度：`P0` / `P1` / `P2` / `P3` |
| scope | VARCHAR(16) | N | 影响范围：`platform` / `vendor` / `tenant` / `project` |
| scope_id | VARCHAR(64) | N | 对应scope的ID，如tenant_id或vendor_name |
| detection_rule | VARCHAR(128) | N | 触发的检测规则名称 |
| metric_name | VARCHAR(64) | N | 异常指标名称 |
| current_value | DOUBLE | N | 当前异常值 |
| baseline_value | DOUBLE | N | 基线值（均值/EMA/历史同期值） |
| deviation_ratio | DOUBLE | N | 偏差比例，如`3.5`表示超出基线350% |
| affected_trace_ids | JSONB | Y | 与异常关联的代表性Trace ID列表（最多100条）|
| detected_at | TIMESTAMP(3) | N | 异常检测时间 |
| resolved_at | TIMESTAMP(3) | Y | 异常解除时间，NULL表示尚未解除 |
| duration_s | INT | Y | 异常持续时长（秒），resolved_at有值时计算 |
| auto_resolved | BOOLEAN | N | 是否自动解除（指标恢复正常触发） |
| notification_sent | BOOLEAN | N | 是否已发送通知 |
| notification_at | TIMESTAMP(3) | Y | 通知发送时间 |
| root_cause_hint | TEXT | Y | 系统生成的根因推断提示 |
| acknowledged_by | VARCHAR(64) | Y | 确认处理的用户ID |
| acknowledged_at | TIMESTAMP(3) | Y | 确认时间 |
| resolution_note | TEXT | Y | 处理备注 |

### 6.4 异常聚类算法

当短时间内出现大量异常事件时，需要聚类合并，避免告警轰炸。聚类策略：

**基于维度的规则聚类：**

在5分钟滑动窗口内，满足以下任一条件的异常事件自动归并为一个异常组（AnomalyGroup）：

- 相同`error_code` + 相同`vendor_name`：供应商级错误聚类
- 相同`error_code` + 相同`model_id`：模型级错误聚类
- 相同`api_key_id` + 相同`anomaly_type`：API Key级异常聚类
- 相同`tenant_id` + 相同`anomaly_type`：租户级异常聚类

**聚类组合并规则：**

- 首个异常事件触发告警（P0/P1即时通知，P2/P3进入汇总）
- 后续同组异常不再单独告警，而是更新组的计数和影响范围
- 若异常组持续超过30分钟未解除，升级一个严重等级后再次通知
- 异常组解除时发送一次"恢复通知"

### 6.5 异常告警分级

| 级别 | 定义 | 通知延迟 | 升级机制 |
|------|------|---------|---------|
| P0 | 全平台不可用或关键供应商全部失联 | 即时（< 1分钟）| 自动电话呼叫值班人员 |
| P1 | 成功率 < 95% 或 P95延迟 > 5s 持续 > 5分钟 | < 2分钟 | 企业微信/钉钉即时消息 |
| P2 | 成功率 < 99% 或 单租户成本异常 | < 5分钟 | 邮件 + 消息 |
| P3 | 潜在风险/趋势预警 | 汇总（每小时一封）| 邮件汇总 |

### 6.6 异常排查引导

平台提供异常事件的根因排查引导流程，帮助运维人员快速定位问题：

```
异常事件详情页
    ↓
[影响分析] 哪些租户/项目/API Key受影响
    ↓
[关联Trace] 查看异常期间代表性Trace（随机采样10条）
    ↓
[Trace Span分析] 哪个Span的耗时占比最大
    ↓
[供应商状态] 查看对应供应商的实时健康状态和历史事故
    ↓
[路由建议] 是否建议临时调整路由策略绕开异常供应商
    ↓
[操作日志] 记录本次异常的处理动作
```

---

## 第7章 Trace审计能力

### 7.1 Trace数据的审计价值

在企业级AI平台的采购决策中，审计能力是一个关键门槛需求，尤其对以下客户类型：

**金融行业客户**：监管机构（如银保监、证监会）要求金融机构对AI系统的使用记录保存至少6个月，且记录不可篡改，以便在发生纠纷或监管检查时提供完整的操作证明。

**政务客户**：政府采购AI服务需要满足等保三级/四级要求，操作日志的完整性和不可篡改性是等保合规的必要条件。

**大型企业**：企业内部审计部门需要验证AI使用费用与供应商账单的一致性，防范API Key泄露导致的未授权使用。

MaaS平台的Trace审计能力提供两类审计证明：

1. **计费准确性证明**：通过Trace的pricing_snapshot字段，证明计费时使用的价格是当时生效的合同价格，而非事后修改的价格。input_tokens和output_tokens字段由供应商返回值和平台估算值双重记录，确保Token数量可核对。

2. **合规策略执行证明**：通过compliance_policy_id和content_safety_result字段，证明平台对每条请求执行了配置的合规策略，为监管报告提供技术依据。

### 7.2 审计级Trace与性能级Trace的分层

并非所有Trace字段都需要以审计标准保存。将Trace分为两个存储层次：

| 属性 | 性能级Trace | 审计级Trace |
|------|-----------|-----------|
| 目的 | 问题排查、性能优化 | 合规证明、账单核对 |
| 保留时长 | 7~30天（按分级） | 强制保留至合规要求（默认180天）|
| 可删除 | 可按需清理 | 不可删除，仅可在到期后归档 |
| 字段范围 | 全量字段 | 审计最小字段集（见下） |
| 存储格式 | ClickHouse行存 | 对象存储NDJSON，Parquet压缩 |
| 写入方式 | 流式写入 | append-only，禁止UPDATE/DELETE |
| 哈希验证 | 无 | 每条记录计算SHA-256哈希，存入哈希链 |

**审计最小字段集：**

trace_id / tenant_id / project_id / api_key_id / model_id / vendor_backend_id / vendor_model_name / request_started_at / completed_at / input_tokens / output_tokens / total_cost_cny / pricing_snapshot / status / http_status_code / error_code / compliance_policy_id / content_safety_result / audit_hash（本条记录的SHA-256哈希）/ prev_audit_hash（前一条审计记录的哈希，形成哈希链）

### 7.3 审计不可篡改保障

**Append-only写入**：审计Trace表使用Object Storage的追加写入能力，不支持覆盖和删除操作。存储层面通过IAM策略限制写操作只允许平台服务账号执行，禁止DELETE和UPDATE。

**哈希链验证**：每条审计Trace记录包含两个哈希字段：

```
audit_hash = SHA256(
  trace_id || tenant_id || project_id || api_key_id || 
  request_started_at || input_tokens || output_tokens || 
  total_cost_cny || pricing_snapshot
)

prev_audit_hash = 前一条（按写入顺序）审计记录的 audit_hash
```

通过哈希链，任何对历史记录的篡改（即使修改单个字段）都会导致哈希链断裂，可被检测到。

**定期完整性校验**：系统每天执行一次哈希链完整性检查，若发现断链则触发P0告警并自动暂停对应数据的后续写入，等待人工核查。

### 7.4 审计Trace查询权限

审计Trace查询是高权限操作，遵循以下权限策略：

| 角色 | 查询范围 | 下载权限 | 删除权限 |
|------|---------|---------|---------|
| 平台审计员（Auditor） | 平台全量审计Trace | CSV/JSON/NDJSON | 无 |
| 租户审计员 | 本租户审计Trace | CSV/JSON | 无 |
| 系统管理员（Super Admin）| 全量 | 全格式 | 仅归档操作（到期后）|
| 其他角色 | 无权限 | 无 | 无 |

查询审计Trace的操作本身会生成操作日志（操作日志不可查询审计Trace数据，避免循环引用）。

### 7.5 Trace导出格式

| 格式 | 适用场景 | 说明 |
|------|---------|------|
| CSV | 导入Excel进行财务核对 | 扁平化字段，JSON子对象序列化为字符串列 |
| JSON | 系统间数据交换 | 标准JSON数组，每条Trace一个对象 |
| NDJSON | 大规模数据处理 | 每行一个JSON对象，适合流式处理和大文件传输 |
| Parquet | 大数据平台分析 | 列式存储格式，支持按列压缩，适合数仓导入 |

---

## 第8章 Trace数据保留策略

### 8.1 保留策略设计原则

数据保留策略的设计需要平衡三组矛盾：

- **存储成本 vs 查询可用性**：保留时间越长，成本越高；但数据删除后无法追溯。
- **隐私保护 vs 业务需要**：Prompt和Response内容可能包含用户个人信息，但业务分析可能需要这些内容。
- **监管要求 vs 技术实现**：不同行业和地区的合规要求差异很大，需要灵活配置。

### 8.2 默认保留策略配置

| 数据类型 | 默认保留时长 | 可配置范围 | 备注 |
|--------|-----------|---------|------|
| 完整Trace（含Prompt/Response） | 7天 | 0~30天 | 0表示不存储内容 |
| Trace元数据（不含内容）| 90天 | 30~365天 | 不包含Prompt/Response |
| 聚合统计数据（小时/天/月粒度）| 2年 | 1~5年 | 纯统计数字，无内容 |
| 审计级Trace | 180天 | 90天~10年 | 受合规要求约束 |
| 告警事件 | 1年 | 90天~5年 | 含告警历史便于趋势分析 |
| 操作审计日志 | 3年 | 1~10年 | 平台操作行为日志 |

### 8.3 按数据分级的保留策略详细规格

**热数据（0~7天）**

- 存储介质：ClickHouse高性能本地NVMe SSD
- 包含字段：trace表全量字段，含trace_spans和route_explanation
- 是否含Prompt/Response：是（除非开启零数据保留模式）
- 查询延迟目标：P95 < 2秒
- 删除机制：ClickHouse TTL自动删除（保留版本升为温数据时保留元数据）
- 访问权限：项目成员及以上角色

**温数据（8~30天）**

- 存储介质：ClickHouse分层存储（温存储磁盘，成本较热存储低60%）
- 包含字段：trace表核心字段（约20个），不含trace_spans详情和Prompt/Response
- 是否含Prompt/Response：否（从热数据降级时执行脱敏迁移）
- 查询延迟目标：P95 < 10秒
- 删除机制：ClickHouse TTL自动转移至冷存储
- 访问权限：项目负责人及以上角色

**冷数据（31~180天）**

- 存储介质：对象存储（OSS/S3），按Parquet格式压缩存储，压缩率约80%
- 包含字段：审计最小字段集（15个字段）
- 是否含Prompt/Response：否
- 查询延迟目标：P95 < 60秒（按需从OSS加载）
- 删除机制：对象存储生命周期规则自动删除
- 访问权限：租户审计员及以上角色

### 8.4 零数据保留模式

零数据保留模式（Zero-Content Retention Mode）是为数据主权极度敏感的客户设计的特殊配置：

**启用效果：**

- Prompt内容（messages数组）在Gateway Service处理完成后立即丢弃，不写入任何存储
- Response内容在转发给客户端后立即丢弃
- trace_spans中不记录input/output内容
- route_explanation中不记录候选模型评分的输入特征

**保留内容：**

- 所有计量字段（Token数、成本、延迟）
- 所有路由决策元数据（不含内容特征）
- 所有错误信息（不含错误中涉及的内容）
- 内容安全检测结论（"命中"/"未命中"），不保留检测器的原始输入

**启用要求：**

- 仅租户管理员（Tenant Admin）可为本租户启用
- 启用后立即生效，不可撤销已存储数据的删除
- 启用状态会在API响应头`x-content-retained: false`中返回，告知调用方

### 8.5 合规要求的最低保留时长

| 合规框架/行业 | 要求的最低保留时长 | 字段范围 |
|------------|----------------|---------|
| 中国网络安全法（通用）| 6个月 | 操作日志 |
| 金融行业（银行/证券）| 5年 | 业务操作记录 |
| 政务（等保三级）| 6个月 | 系统访问日志 |
| 医疗（卫生行业标准）| 3年 | 业务数据访问记录 |
| GDPR（欧盟，跨境业务）| 保留期与处理目的挂钩 | 含个人数据的处理记录 |
| SOC2 Type II | 1年 | 安全相关事件日志 |

平台提供合规模式预设（如"金融合规模式"），自动将保留策略配置为对应合规要求的最低标准。

---

## 第9章 告警规则配置

### 9.1 告警规则数据模型

```sql
CREATE TABLE maas.alert_rules (
    rule_id         VARCHAR(32) PRIMARY KEY,
    tenant_id       VARCHAR(32) NOT NULL,  -- 平台级告警为空字符串
    name            VARCHAR(128) NOT NULL,
    description     TEXT,
    alert_type      VARCHAR(32) NOT NULL,  -- threshold/trend/anomaly/sla
    scope           VARCHAR(32) NOT NULL,  -- platform/tenant/project/vendor
    scope_id        VARCHAR(64),           -- 作用域ID，platform级为空
    metric          VARCHAR(64) NOT NULL,  -- 监控指标名称
    aggregation     VARCHAR(16) NOT NULL,  -- avg/max/min/sum/count/p95/p99
    window_minutes  INT NOT NULL,          -- 评估窗口（分钟）
    condition       VARCHAR(8) NOT NULL,   -- gt/gte/lt/lte/eq/neq
    threshold       DOUBLE NOT NULL,       -- 告警阈值
    severity        VARCHAR(4) NOT NULL,   -- P0/P1/P2/P3
    notify_channels JSONB NOT NULL,        -- 通知渠道配置
    snooze_minutes  INT DEFAULT 30,        -- 告警静默时间（分钟）
    enabled         BOOLEAN DEFAULT TRUE,
    created_by      VARCHAR(64) NOT NULL,
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP
);
```

### 9.2 默认告警模板

平台提供开箱即用的默认告警模板，租户可直接启用或在此基础上调整阈值：

| 模板名称 | 指标 | 聚合 | 窗口 | 条件 | 默认阈值 | 级别 |
|--------|------|------|------|------|---------|------|
| 错误率超限 | error_rate | avg | 5分钟 | gt | 5% | P1 |
| P95延迟超限 | p95_latency_ms | max | 5分钟 | gt | 3000ms | P1 |
| TTFB超限 | p95_ttfb_ms | max | 5分钟 | gt | 1000ms | P2 |
| Fallback率超限 | fallback_rate | avg | 10分钟 | gt | 10% | P1 |
| 每小时成本超限 | hourly_cost_cny | sum | 60分钟 | gt | 租户预算/720（月均） | P1 |
| 成功率过低 | success_rate | avg | 5分钟 | lt | 95% | P0 |
| 缓存命中率下降 | cache_hit_rate | avg | 30分钟 | lt | 10% | P3 |
| 供应商全部失联 | vendor_health_score | min | 1分钟 | lt | 0.1 | P0 |
| 配额即将耗尽 | quota_remaining_pct | min | 实时 | lt | 10% | P1 |
| Token暴增 | token_rate_rpm | avg | 5分钟 | gt | 历史P99×3 | P1 |

### 9.3 告警通知渠道配置

**通知渠道类型及配置字段：**

| 渠道类型 | 配置字段 | 说明 |
|--------|---------|------|
| 邮件（Email）| `to`（收件人列表）、`cc`（抄送）| 支持多收件人，富文本格式 |
| 企业微信机器人 | `webhook_url`、`mentioned_list`（@成员）| 通过企业微信机器人Webhook |
| 钉钉机器人 | `webhook_url`、`at_mobiles`（@手机号）| 通过钉钉机器人Webhook |
| 通用Webhook | `url`、`method`（POST）、`headers`、`body_template` | 自定义请求格式，支持Jinja2模板 |
| 短信（SMS） | `phone_numbers`（手机号列表） | 通过平台短信网关，P0/P1级别可用 |
| PagerDuty | `integration_key` | 集成PagerDuty的Events API v2 |
| Slack | `webhook_url`、`channel` | 通过Slack Incoming Webhook |

### 9.4 告警降噪机制

**静默窗口（Snooze）**：告警触发后，在snooze_minutes分钟内相同规则不再重复发送通知，避免告警风暴。

**聚合通知**：P3级别的告警不即时发送，而是每小时汇总一次，将一小时内的所有P3告警合并为一封邮件发送。

**恢复通知**：当告警条件解除（指标恢复正常）时，发送"恢复通知"，注明告警持续时长和解除时间。

**重复抑制**：若某条告警规则已处于告警状态（尚未解除），即使窗口内再次触发条件，不重复发送通知（除非已经过了snooze时间）。

**告警分组**：同一时间窗口内来自同一scope（如同一供应商）的多个告警，自动合并为一条通知，在通知中列举所有触发的规则。

### 9.5 告警操作记录

所有告警事件记录在alert_events表，支持以下操作：

- **确认（Acknowledge）**：标记告警已被接收处理，停止升级计时
- **静默（Mute）**：将特定告警规则临时静默N分钟（运维窗口期间常用）
- **关闭（Close）**：手动关闭告警（当自动解除逻辑未触发时）
- **添加备注（Comment）**：记录处理过程和根因分析
- **创建事故单（Create Incident）**：将告警关联到事故管理系统（Jira/飞书任务/自定义）

---

## 第10章 Trace API与SDK集成

### 10.1 Trace查询API端点设计

**基础URL**：`https://api.maas-platform.com/v1/traces`

#### 10.1.1 查询Trace列表

```
GET /v1/traces
Authorization: Bearer {management_api_key}

Query Parameters:
  tenant_id      string    租户ID（平台级查询必填）
  project_id     string    项目ID过滤
  model_id       string    逻辑模型ID过滤
  vendor_name    string    供应商名称过滤
  status         string    状态过滤（支持多值，逗号分隔）
  start_time     ISO8601   查询开始时间（必填）
  end_time       ISO8601   查询结束时间（必填）
  session_id     string    按Session过滤
  api_key_id     string    按API Key过滤
  has_fallback   boolean   是否仅返回有fallback的请求
  tag_{key}      string    按业务标签过滤，如 tag_env=prod
  page           int       页码，默认1
  page_size      int       每页条数，默认20，最大100
  sort           string    排序字段，如 -request_started_at（降序）

Response 200:
{
  "total": 12580,
  "page": 1,
  "page_size": 20,
  "data": [ { ...trace对象 } ]
}
```

#### 10.1.2 获取单条Trace详情

```
GET /v1/traces/{trace_id}
Authorization: Bearer {management_api_key}

Response 200:
{
  "trace_id": "trc_01HZ...",
  "request_id": "req_abc123",
  "session_id": "ses_xyz789",
  ...（完整trace对象，含trace_spans和route_explanation）
}
```

#### 10.1.3 Trace聚合统计接口

```
POST /v1/traces/aggregate
Authorization: Bearer {management_api_key}
Content-Type: application/json

Request Body:
{
  "tenant_id": "tenant_abc",
  "start_time": "2026-05-21T00:00:00Z",
  "end_time": "2026-05-21T23:59:59Z",
  "group_by": ["project_id", "model_id"],
  "metrics": ["request_count", "success_rate", "p95_ttfb_ms", "total_cost_cny"],
  "filters": {
    "vendor_name": ["openai", "anthropic"],
    "status": ["success", "fallback_success"]
  }
}

Response 200:
{
  "groups": [
    {
      "project_id": "proj_123",
      "model_id": "gpt-4o",
      "request_count": 5840,
      "success_rate": 0.9923,
      "p95_ttfb_ms": 876,
      "total_cost_cny": 234.56
    }
  ]
}
```

### 10.2 请求头传递规范

客户端在调用MaaS平台的LLM API时，通过以下自定义请求头传递观测元数据：

| 请求头 | 类型 | 必填 | 说明 |
|-------|------|------|------|
| `x-request-id` | string | 推荐 | 客户端生成的唯一请求ID，平台将其存入trace.request_id。若不传，平台自动生成 |
| `x-session-id` | string | 可选 | 会话ID，用于多轮对话关联。格式建议：`ses_` + UUID，长度不超过64字符 |
| `x-trace-tags` | JSON字符串 | 可选 | 业务标签，URL编码的JSON对象，如`{"env":"prod","feature":"chat","user_tier":"premium"}`。标签Key最多20个，Value最长64字符 |
| `x-user-id` | string | 可选 | 终端用户标识，平台不校验其合法性，仅透传存储。最长256字符 |
| `x-idempotency-key` | string | 可选 | 幂等Key，相同Key在60秒内重复发送返回第一次的响应，用于客户端重试 |

**响应头（平台返回）：**

| 响应头 | 说明 |
|-------|------|
| `x-trace-id` | 本次请求的Trace ID，便于客户端记录日志 |
| `x-request-id` | 回显客户端传入的request-id，若客户端未传则为平台生成值 |
| `x-cache-hit` | `true`/`false`，是否命中缓存 |
| `x-fallback-used` | `true`/`false`，是否触发了fallback |
| `x-model-used` | 实际使用的供应商侧模型名称 |
| `x-content-retained` | `true`/`false`，内容是否被保留（零数据保留模式下为`false`）|

### 10.3 SDK集成示例

#### Python SDK

```python
from maas_client import MaaSClient

client = MaaSClient(
    api_key="sk-xxx",
    base_url="https://api.maas-platform.com/v1",
    # SDK自动生成x-request-id，也可手动指定
    default_headers={
        "x-trace-tags": '{"env":"prod","feature":"chat-assistant"}'
    }
)

# 多轮对话示例，传入session_id实现会话关联
response = client.chat.completions.create(
    model="gpt-4o",
    messages=[
        {"role": "user", "content": "你好，帮我分析一下这份合同"}
    ],
    extra_headers={
        "x-session-id": "ses_user_123_conversation_456",
        "x-user-id": "end_user_789"
    }
)

# 从响应头获取Trace ID，记录到应用日志
trace_id = response.headers.get("x-trace-id")
print(f"Trace ID: {trace_id}")
```

#### TypeScript/Node.js SDK

```typescript
import { MaaSClient } from '@maas-platform/sdk';

const client = new MaaSClient({
  apiKey: process.env.MAAS_API_KEY,
  baseURL: 'https://api.maas-platform.com/v1',
  defaultHeaders: {
    'x-trace-tags': JSON.stringify({ env: 'prod', feature: 'chatbot' })
  }
});

async function chat(sessionId: string, userId: string, message: string) {
  const response = await client.chat.completions.create({
    model: 'claude-3-5-sonnet',
    messages: [{ role: 'user', content: message }],
  }, {
    headers: {
      'x-session-id': sessionId,
      'x-user-id': userId,
    }
  });

  const traceId = response.headers?.['x-trace-id'];
  console.log(`Request traced: ${traceId}`);
  return response.choices[0].message.content;
}
```

#### Java SDK

```java
import com.maasplatform.sdk.MaaSClient;
import com.maasplatform.sdk.model.ChatCompletionRequest;

MaaSClient client = MaaSClient.builder()
    .apiKey(System.getenv("MAAS_API_KEY"))
    .baseUrl("https://api.maas-platform.com/v1")
    .defaultHeader("x-trace-tags", "{\"env\":\"prod\",\"service\":\"recommendation\"}")
    .build();

ChatCompletionRequest request = ChatCompletionRequest.builder()
    .model("qwen-max")
    .addMessage("user", "请推荐三款适合家庭的理财产品")
    .header("x-session-id", "ses_" + sessionId)
    .header("x-user-id", userId)
    .build();

var response = client.chat().completions().create(request);
String traceId = response.getHeader("x-trace-id");
```

### 10.4 OpenTelemetry兼容设计

MaaS平台的Trace体系与OpenTelemetry（OTel）规范保持兼容，便于企业客户将MaaS的Trace数据集成到现有的可观测性平台（如Jaeger、Zipkin、Grafana Tempo）。

**OTel集成方式：**

1. **作为OTel Exporter**：Gateway Service在处理每个请求时，同时向客户配置的OTel Collector发送Span数据，使MaaS的请求Trace出现在企业的分布式追踪系统中。

2. **W3C TraceContext传播**：MaaS平台支持读取和传播`traceparent`和`tracestate`头，将客户端的OTel Trace上下文延续到MaaS内部，形成从客户端到供应商的完整调用链。

3. **OTel语义规范对齐**：

| OTel属性 | MaaS Trace字段 | 说明 |
|---------|--------------|------|
| `trace.id` | trace_id | OTel Trace ID映射 |
| `span.id` | span_id | OTel Span ID映射 |
| `http.method` | POST | LLM请求均为POST |
| `http.status_code` | http_status_code | HTTP响应码 |
| `llm.model` | model_id | LLM模型语义属性 |
| `llm.input_tokens` | input_tokens | Token数量 |
| `llm.output_tokens` | output_tokens | Token数量 |
| `llm.vendor` | vendor_name | 供应商属性 |

**OTel导出配置示例（平台管理控制台）：**

```yaml
# 在租户设置 > 可观测性 > OTel集成中配置
otel_export:
  enabled: true
  endpoint: "https://otel-collector.your-company.com:4317"
  protocol: grpc  # grpc 或 http/protobuf
  headers:
    Authorization: "Bearer your-collector-token"
  export_traces: true
  export_metrics: true
  sampling_rate: 0.1  # 仅导出10%的Trace（成本控制）
  include_prompt_content: false  # 是否在OTel Span中包含Prompt内容
```

---

## 第11章 验收标准

### 11.1 功能验收标准

| 验收项 | 验收标准 | 验收方法 |
|-------|---------|---------|
| Trace完整性 | 每个完成的请求（含错误请求）均生成唯一Trace记录，Trace完整率 > 99.99% | 注入1万条测试请求，对比请求数与Trace数 |
| trace_spans准确性 | 所有span_type的duration_ms之和 ≤ total_latency_ms（允许5ms误差）| 随机抽取100条Trace验证Span时间一致性 |
| route_explanation完整性 | fallback_triggered=true的Trace，其route_explanation.fallback_chain字段不为空 | 强制触发100次fallback，检查route_explanation |
| Session关联准确性 | 携带相同x-session-id的连续请求，正确聚合到同一Session | 发送10组多轮对话（每组5轮），验证Session聚合 |
| 成本归因精度 | 聚合成本与各明细Trace成本之和误差 < 0.001 CNY | 对比cost_hourly_agg与traces表的SUM值 |
| 缓存命中正确标记 | 命中语义缓存的请求，trace.cache_hit=true且total_cost_cny=0 | 发送相同内容的请求，第二次验证cache_hit |
| 审计哈希链完整 | 全量审计Trace的哈希链无断裂 | 调用哈希链验证接口，应返回VALID |
| 零数据保留模式 | 启用零数据保留后，ClickHouse中对应租户无Prompt/Response内容 | 启用后发送请求，直接查询ClickHouse确认 |
| 告警触发延迟 | P0/P1告警从触发到通知 < 2分钟 | 模拟错误率超限，计时通知到达 |
| OTel导出 | 配置OTel Collector后，Trace数据在30秒内出现在Jaeger等系统 | 配置测试Collector，验证数据到达 |

### 11.2 性能验收标准

| 验收项 | 标准值 | 测试条件 |
|-------|-------|---------|
| Trace写入延迟 | Kafka消费到ClickHouse写入完成 < 3秒（P95）| 持续压测，1000 TPS |
| Trace查询延迟（热数据）| 单Trace详情查询 P95 < 500ms | 并发100请求查询最近7天数据 |
| 聚合查询延迟（热数据）| 按tenant+project+day维度聚合 P95 < 2秒 | 含1000万条Trace的表 |
| Session聚合延迟 | Session视图载入 P95 < 3秒 | 含100轮对话的Session |
| 告警检测时延 | 异常发生到告警事件生成 < 1分钟 | 注入批量错误请求 |
| 仪表板刷新延迟 | 实时概览看板数据延迟 < 30秒 | 与实际请求时间对比 |
| Trace API QPS | Trace查询API支持 > 100 QPS | 并发压测 |
| 哈希链验证性能 | 100万条审计Trace的哈希链完整性校验 < 10分钟 | 离线批量验证任务 |

### 11.3 可靠性验收标准

| 验收项 | 标准 |
|-------|------|
| Trace丢失率 | < 0.001%（十万分之一）|
| Kafka消费积压恢复 | 积压100万条消息时，2小时内消费完成 |
| ClickHouse写入失败重试 | 写入失败自动重试3次，3次失败后写入死信队列，运维告警 |
| 审计Trace持久化保证 | 即使ClickHouse不可用，审计Trace先写入Kafka持久化，不丢失 |
| 数据保留策略执行 | TTL删除操作不影响当时的查询性能，删除期间P99查询延迟增加 < 20% |

### 11.4 安全验收标准

| 验收项 | 标准 |
|-------|------|
| Trace数据访问控制 | 租户A无法访问租户B的任何Trace数据（租户隔离测试）|
| 审计Trace不可篡改 | 数据库层面无法执行UPDATE/DELETE操作（服务账号权限验证）|
| 敏感字段脱敏 | 温数据和冷数据中无Prompt/Response明文（随机采样检查）|
| API Key脱敏 | Trace中存储api_key_id而非Key明文，查询接口不返回Key明文 |
| Trace查询操作日志 | 每次Trace查询记录操作审计日志，覆盖率100% |

---

## 附录A：指标名称与计算口径速查表

| 指标名称 | 中文名 | 计算公式 | 数据来源 |
|--------|------|---------|---------|
| `request_count` | 请求总数 | COUNT(trace_id) | traces表 |
| `success_rate` | 成功率 | COUNT(status='success') / COUNT(*) | traces表 |
| `error_rate` | 错误率 | COUNT(status!='success') / COUNT(*) | traces表 |
| `p50_ttfb_ms` | TTFB中位数 | QUANTILE(0.5)(ttfb_ms) | traces表（仅流式请求）|
| `p95_ttfb_ms` | TTFB P95 | QUANTILE(0.95)(ttfb_ms) | traces表（仅流式请求）|
| `p99_latency_ms` | 总延迟P99 | QUANTILE(0.99)(total_latency_ms) | traces表 |
| `avg_input_tokens` | 平均输入Token | AVG(input_tokens) | traces表 |
| `total_cost_cny` | 总成本 | SUM(total_cost_cny) | traces表 |
| `fallback_rate` | Fallback率 | COUNT(fallback_triggered=1) / COUNT(*) | traces表 |
| `cache_hit_rate` | 缓存命中率 | COUNT(cache_hit=1) / COUNT(*) | traces表 |
| `cache_saved_cost_cny` | 缓存节省费用 | SUM(cache_saved_cost_cny) | traces表 |
| `avg_session_turns` | 平均会话轮次 | AVG(turn_count) | sessions表 |
| `avg_session_cost` | 平均会话成本 | AVG(total_cost_cny) | sessions表 |
| `anomaly_count` | 异常事件数 | COUNT(anomaly_id) | anomaly_events表 |
| `vendor_health_score` | 供应商健康评分 | 综合成功率+延迟+错误率的加权评分 | 实时计算 |

---

## 第11章 Agent Trace 专节

> **背景**：随着 Agent（智能体）应用在企业内快速落地，单次"请求-响应"的 Trace 模型已无法完整描述多步骤、工具调用、RAG 检索、多模型协作等复杂调用链路。本章定义 Agent 场景下的 Trace 扩展数据模型与视图规格，作为 GAP-28 的完整覆盖。

---

### 11.1 Agent Trace 的核心挑战

普通 LLM 调用是一个单步 Span：`请求 → 路由 → 供应商调用 → 响应`。Agent 调用是一棵 **Span 树**，典型结构如下：

```
Agent Run (root span)
├── Step 1: LLM 推理（选择工具）
│   └── 供应商调用 Span
├── Step 2: Tool Call - search_knowledge_base
│   ├── RAG 检索 Span（向量数据库查询）
│   └── 结果注入 Span
├── Step 3: LLM 推理（基于检索结果生成答案）
│   └── 供应商调用 Span
└── Step 4: Tool Call - send_email（最终动作）
    └── 外部 API 调用 Span
```

**挑战一**：每个 Agent Run 包含 N 次 LLM 调用，成本必须在 Run 维度聚合，同时保留每步 Token 明细。  
**挑战二**：工具调用（Tool Call）不经过 MaaS 供应商层，但属于 Agent 执行链路，需要在 Trace 中记录工具名称、输入参数、执行耗时、输出摘要。  
**挑战三**：RAG 检索步骤可能涉及 Embedding 模型调用（经过 MaaS）和向量库查询（不经过 MaaS），两类 Span 需要区分。  
**挑战四**：多轮对话中，一个 Session 可能包含多个 Agent Run，需要在 Session 维度关联所有 Run。

---

### 11.2 Agent Trace 数据模型扩展

在现有 `trace_span` 表基础上，新增以下字段支撑 Agent Trace：

```sql
-- 在 trace_span 表新增 Agent 相关字段
ALTER TABLE trace_span ADD COLUMN IF NOT EXISTS
    span_type           VARCHAR(32)   DEFAULT 'LLM_CALL',
    -- 枚举值：LLM_CALL | TOOL_CALL | RAG_RETRIEVAL | EMBEDDING | AGENT_RUN | RERANK | EXTERNAL_API

    agent_run_id        VARCHAR(128),  -- 所属 Agent Run 的根 Span ID
    parent_span_id      VARCHAR(128),  -- 父 Span ID（支持多层嵌套）
    step_index          INT,           -- 在当前 Agent Run 中的步骤序号（从 1 开始）
    step_name           VARCHAR(256),  -- 人可读步骤名称，如 "检索知识库" / "生成回答"

    -- Tool Call 专用字段（span_type = TOOL_CALL 时填充）
    tool_name           VARCHAR(128),  -- 工具名称，如 "search_knowledge_base" / "send_email"
    tool_input_hash     VARCHAR(64),   -- 工具输入参数的 SHA256 哈希（不存明文，防止敏感数据泄漏）
    tool_output_size    INT,           -- 工具返回内容的字节数
    tool_status         VARCHAR(32),   -- SUCCESS | FAILED | TIMEOUT | SKIPPED

    -- RAG 专用字段（span_type = RAG_RETRIEVAL 时填充）
    retrieval_query_hash VARCHAR(64),  -- 检索 Query 的哈希
    retrieved_doc_count  INT,          -- 命中文档数量
    retrieval_score_max  DECIMAL(5,4), -- 最高相似度分
    retrieval_score_min  DECIMAL(5,4), -- 最低相似度分

    -- Agent Run 聚合字段（span_type = AGENT_RUN 时填充，为根 Span）
    total_llm_calls     INT,           -- 本次 Run 中 LLM_CALL 类型 Span 总数
    total_tool_calls    INT,           -- 本次 Run 中 TOOL_CALL 类型 Span 总数
    total_input_tokens  INT,           -- 本次 Run 所有 LLM 调用的 input token 合计
    total_output_tokens INT,           -- 本次 Run 所有 LLM 调用的 output token 合计
    total_cost_usd      DECIMAL(12,8), -- 本次 Run 所有 LLM 调用的成本合计
    agent_status        VARCHAR(32),   -- COMPLETED | FAILED | INTERRUPTED | MAX_STEPS_EXCEEDED
    max_steps_allowed   INT,           -- 配置的最大步骤数
    actual_steps        INT;           -- 实际执行步骤数
```

---

### 11.3 Agent Run 视图规格

**页面入口**：Console → Trace 调用明细 → `D5E - Agent Run 视图`（新增导航项）

**视图一：Agent Run 列表**

| 字段 | 说明 |
|------|------|
| Run ID | 根 Span ID，可点击展开 |
| 发起时间 | Agent Run 开始时间戳 |
| 总耗时 | 从 Run 开始到结束的端到端时长 |
| 总步骤数 | 实际执行步骤数 / 最大允许步骤数 |
| LLM调用次数 | 本次 Run 中 LLM 调用总数 |
| 总Token消耗 | input + output token 合计 |
| 总成本 | 折算为人民币/美元，支持切换 |
| 状态 | COMPLETED / FAILED / MAX_STEPS_EXCEEDED |
| 关联 Session | 所属 Session ID（可跳转 Session 视图） |

**视图二：Agent Run 瀑布图（Waterfall）**

选中某个 Agent Run 后，展开为可视化瀑布图，规格如下：

- 横轴：时间轴（ms）
- 纵轴：Span 层级树（缩进展示父子关系）
- 每个 Span 的颜色区分类型：
  - 🔵 蓝色：`LLM_CALL`（LLM 推理）
  - 🟢 绿色：`TOOL_CALL`（工具调用）
  - 🟡 黄色：`RAG_RETRIEVAL`（知识库检索）
  - 🟠 橙色：`EMBEDDING`（向量化）
  - 🔴 红色：失败的 Span
- 点击任意 Span 展开右侧详情面板，包含该 Span 的完整字段
- 瀑布图支持导出为 PNG 或 JSON（供外部分析工具使用）

**视图三：Agent Run 成本分解**

在 Run 详情页底部展示成本饼图：
- 按模型分解：每个逻辑模型的 Token 消耗和成本占比
- 按步骤分解：每个步骤（step_index）的成本贡献
- 提示信息：若存在 `total_tool_calls > 0`，标注"工具调用本身不计费，仅 LLM 推理步骤产生成本"

---

### 11.4 多轮对话中的 Agent Run 关联

一个 Session（`session_id`）可以包含多个 Agent Run。Session 视图（第3章）在展示多轮对话时，需增加以下 Agent 感知：

- **Session 时间线**：将 Agent Run 作为一个"事件块"展示，块内可展开查看步骤树
- **Session 级聚合统计**：
  - 总 Agent Run 数量
  - 总 LLM 调用次数（包括 Agent 内嵌的调用）
  - 总成本（会话维度）
  - 平均每轮 Agent Run 步骤数

---

### 11.5 Agent Trace 的数据采集方式

MaaS 平台侧可以自动捕获所有经过网关的 `LLM_CALL` 和 `EMBEDDING` 类型 Span。`TOOL_CALL` 和 `RAG_RETRIEVAL` 类型 Span 需要客户端主动上报，MaaS 提供以下两种方式：

**方式一：Trace SDK 主动上报**
```python
# Python SDK 示例
from maas import TraceClient

trace = TraceClient(api_key="proj_xxx")

with trace.agent_run(session_id="sess_123") as run:
    with run.step("检索知识库", span_type="RAG_RETRIEVAL") as step:
        results = vector_db.search(query)
        step.set_retrieval_meta(doc_count=len(results), score_max=results[0].score)
    
    with run.step("LLM 生成答案", span_type="LLM_CALL") as step:
        response = maas_client.chat(...)  # MaaS 会自动关联此 Span 到 run
```

**方式二：OpenTelemetry 兼容上报**  
MaaS Trace 接收端兼容 OpenTelemetry OTLP 协议。客户端使用 OTel SDK 上报 Span 时，若 Span 包含 MaaS 定义的 `maas.span_type` Attribute，系统自动解析并关联到对应 Agent Run。

> **隐私保护**：工具输入参数和检索 Query 均只存哈希值，原文不持久化（除非租户显式开启"Prompt内容留存"策略且通过合规审批）。

---

### 11.6 Agent Trace 验收标准

| # | 验收项 | 验收口径 |
|---|--------|---------|
| AT-01 | Span 树完整性 | 一次包含 5 步骤（2次LLM + 2次Tool + 1次RAG）的 Agent Run，瀑布图展示 5 个 Span，父子关系正确，无缺失 |
| AT-02 | 成本聚合准确 | Agent Run 根 Span 的 `total_cost_usd` = 所有 `LLM_CALL` 子 Span 的 `cost_usd` 之和，误差 < 0.01% |
| AT-03 | Session 关联 | 同一 `session_id` 下的多个 Agent Run 在 Session 视图中正确聚合，总成本 = 各 Run 成本之和 |
| AT-04 | 工具调用不计费 | `TOOL_CALL` 类型 Span 的 `cost_usd = 0`，不影响 billing_ledger 记录 |
| AT-05 | SDK 上报延迟 | 客户端通过 Trace SDK 上报的 Span，在上报后 ≤ 5 秒内可在 Console 瀑布图中查询到 |
| AT-06 | 瀑布图导出 | 点击"导出 JSON"后，下载文件包含完整 Span 树结构，可被 Jaeger / Zipkin 导入解析 |
| AT-07 | MAX_STEPS 标记 | Agent 超过 `max_steps_allowed` 时，根 Span 状态标记为 `MAX_STEPS_EXCEEDED`，Console 显示橙色警告 |

---

## 第12章 验收标准

| 错误码 | HTTP状态码 | 说明 | 建议处理 |
|-------|-----------|------|---------|
| `VENDOR_TIMEOUT` | 504 | 供应商API超时 | 触发fallback或重试 |
| `VENDOR_RATE_LIMIT` | 429 | 供应商限流 | 等待后重试或切换后端 |
| `VENDOR_API_ERROR` | 502 | 供应商返回5xx错误 | 触发fallback |
| `QUOTA_EXCEEDED` | 429 | 项目配额已耗尽 | 联系管理员调整配额 |
| `BUDGET_EXCEEDED` | 429 | 预算已耗尽 | 联系管理员增加预算 |
| `RATE_LIMITED` | 429 | 平台层限流（TPM/RPM）| 降低请求频率 |
| `CONTENT_POLICY_VIOLATION` | 400 | 输入内容违反合规策略 | 检查输入内容 |
| `OUTPUT_CONTENT_BLOCKED` | 200+标记 | 输出内容被拦截，返回部分内容 | 检查请求内容 |
| `INPUT_TOO_LONG` | 400 | 输入Token超过模型上下文限制 | 压缩输入内容 |
| `INVALID_MODEL` | 400 | 指定的模型不存在或不可用 | 检查model参数 |
| `AUTH_FAILED` | 401 | API Key无效或已吊销 | 检查API Key |
| `PERMISSION_DENIED` | 403 | API Key无权访问该模型 | 检查API Key权限配置 |
| `ALL_BACKENDS_FAILED` | 503 | 所有fallback后端均失败 | 联系平台运维 |

---

*文档结束 — 共11章 + 2个附录*  
*下一文档：`05-Prompt管理与评测规格.md`*
