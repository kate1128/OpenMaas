# routero-develop 源码走读调研报告

**走读日期：** 2026年05月22日  
**源码路径：** `routero-develop/routero-develop/`  
**基础版本：** LiteLLM（开源 AI Gateway，MIT License）  
**报告目的：** 评估该项目二次开发深度，梳理核心模块，为 MaaS 平台选型提供依据

---

## 一、项目定性

该项目是基于 **LiteLLM 开源代码的二次开发版本（fork）**，在官方代码库基础上新增了以下自研模块：

| 自研模块 | 路径 | 功能 |
|---------|------|------|
| `coworker` | `coworker/` | 独立消费进程：Redis → DB 的 Spend 同步服务，含 Redis Leader Election |
| `vidaimock` | `vidaimock/` | 高性能 LLM Mock Server（集成 VidaiMock），用于路由策略压测 |

其余绝大部分代码来自 LiteLLM 上游，包括供应商适配、路由器、Proxy 网关等核心逻辑。

---

## 二、整体架构

```
外部客户端（OpenAI SDK / Anthropic SDK / 任意 HTTP）
        │ HTTPS
        ▼
┌──────────────────────────────────────────────────────┐
│              LiteLLM AI Gateway (proxy/)              │
│  ┌─────────┐  ┌──────────┐  ┌────────────────────┐  │
│  │  Auth   │  │  Hooks   │  │  Route/Pre-call     │  │
│  │ API Key │  │ 限流/预算 │  │ litellm_pre_call   │  │
│  │ JWT/SSO │  │ Guardrail│  │ route_llm_request  │  │
│  └─────────┘  └──────────┘  └────────────────────┘  │
│                    │                                   │
│         ┌──────────▼──────────┐                       │
│         │   Router (router.py) │                       │
│         │  负载均衡 / Fallback  │                       │
│         └──────────┬──────────┘                       │
└────────────────────┼─────────────────────────────────┘
                     │
┌────────────────────▼─────────────────────────────────┐
│              LiteLLM SDK (litellm/)                   │
│  main.py → llm_http_handler → transform → HTTP       │
│  支持 100+ 供应商（llms/目录下每个子目录一个供应商）   │
└──────────────────────────────────────────────────────┘
         │ PostgreSQL         │ Redis
         ▼                    ▼
    Keys/Teams/SpendLogs   限流/缓存/TPM/RPM/Cooldown

独立进程：
┌────────────────────────────────┐
│  coworker（自研）               │
│  Redis BLPOP → Batch → DB写入  │
│  Redis Leader Election 保证单主 │
└────────────────────────────────┘
```

---

## 三、核心模块详解

### 3.1 请求完整链路

```
POST /v1/chat/completions
  │
  ├─ [1] proxy/auth/user_api_key_auth.py
  │      验证 API Key → Redis 缓存 → PG fallback
  │      支持 JWT / OAuth2 / SSO
  │
  ├─ [2] proxy/hooks/
  │      max_budget_limiter        → 预算上限检查
  │      parallel_request_limiter_v3 → TPM/RPM 限流（Lua 滑动窗口）
  │      cache_control_check       → 缓存有效性
  │
  ├─ [3] proxy/route_llm_request.py
  │      调用 router.py 选择部署实例
  │
  ├─ [4] litellm/router.py
  │      按路由策略选目标（见 3.2）
  │
  ├─ [5] litellm/main.py → acompletion()
  │      调用 BaseLLMHTTPHandler
  │
  ├─ [6] llms/{provider}/chat/transformation.py
  │      transform_request()  OpenAI格式 → 供应商格式
  │      HTTP 请求 → 供应商 API
  │      transform_response() 供应商格式 → OpenAI格式
  │
  ├─ [7] 成本计算
  │      litellm_logging.py._response_cost_calculator()
  │      response._hidden_params["response_cost"] 携带成本
  │
  └─ [8] 异步日志（不阻塞主路径）
         hooks/proxy_track_cost_callback.py
         db/db_spend_update_writer.py → Redis 队列
         coworker（独立进程）→ PostgreSQL 批量写入
```

### 3.2 路由策略（router_strategy/）

| 策略 | 文件 | 算法说明 |
|------|------|---------|
| `lowest-latency` | `lowest_latency.py` | 维护每个 deployment 的滑动延迟均值，选最低延迟实例 |
| `lowest-cost` | `lowest_cost.py` | 按 Token 单价排序，优先选最便宜实例 |
| `lowest-tpm-rpm` | `lowest_tpm_rpm_v2.py` | 基于 Redis 中 TPM/RPM 计数选负载最低实例 |
| `least-busy` | `least_busy.py` | 选当前并发请求数最少的实例 |
| `simple-shuffle` | `simple_shuffle.py` | 加权随机（健康实例中随机选取） |
| `tag-based` | `tag_based_routing.py` | 按请求 metadata 中的 tag 匹配 deployment |
| `auto-router` | `auto_router/auto_router.py` | 语义路由：基于 Prompt 内容用向量相似度选模型（调用 `semantic-router` 库） |

**Cooldown 熔断机制（`router_utils/cooldown_handlers.py`）：**
- 每分钟统计失败率，超过阈值（默认 `DEFAULT_FAILURE_THRESHOLD_PERCENT`）触发 Cooldown
- Cooldown 期间该 deployment 被从候选列表移除
- 默认冷却时间：`DEFAULT_COOLDOWN_TIME_SECONDS`（常量可配置）
- 单部署特殊逻辑：当只有一个 deployment 时适用 `SINGLE_DEPLOYMENT_TRAFFIC_FAILURE_THRESHOLD`

### 3.3 供应商适配层（llms/）

共支持 **100+ 供应商**，每个供应商一个子目录，遵循统一模式：

```python
# 每个供应商的翻译类
class ProviderConfig(BaseConfig):
    def transform_request(self, model, messages, optional_params, ...):
        # OpenAI 格式 → 供应商原生格式
        
    def transform_response(self, model, raw_response, model_response, ...):
        # 供应商格式 → 统一 ModelResponse
```

与 MaaS 平台直接相关的供应商：

| 目录 | 供应商 |
|------|-------|
| `llms/openai/` | OpenAI / Azure OpenAI（OpenAI-兼容） |
| `llms/anthropic/` | Anthropic Claude |
| `llms/dashscope/` | 阿里云百炼 / 通义千问 |
| `llms/deepseek/` | DeepSeek |
| `llms/moonshot/` | 月之暗面 |
| `llms/volcengine/` | 字节豆包 |
| `llms/zhipuai/` | 智谱 AI（注：目录名可能为 zhipuai 或 openai_like） |
| `llms/gemini/` | Google Gemini |
| `llms/bedrock/` | AWS Bedrock（含 Claude/Llama/Mistral） |
| `llms/vertex_ai/` | Google Vertex AI |

### 3.4 缓存层（caching/）

| 缓存类型 | 文件 | 用途 |
|---------|------|------|
| `InMemoryCache` | `in_memory_cache.py` | 本地 LRU（单实例热路径） |
| `RedisCache` | `redis_cache.py` | 分布式缓存、限流计数器 |
| `DualCache` | `dual_cache.py` | 内存 + Redis 两级，先查内存再查 Redis |
| `RedisSemanticCache` | `redis_semantic_cache.py` | Redis Vector Search 语义缓存 |
| `QdrantSemanticCache` | `qdrant_semantic_cache.py` | Qdrant 语义缓存，支持相似度阈值和 Scalar 量化 |
| `S3Cache` | `s3_cache.py` | S3/MinIO 冷缓存 |
| `DiskCache` | `disk_cache.py` | 本地磁盘（开发/测试用） |

语义缓存命中条件与 MaaS 平台 adapter-service 设计一致：`temperature=0`，内容哈希 + 向量相似度双重校验。

### 3.5 Auth 与多租户（proxy/auth/）

| 文件 | 功能 |
|------|------|
| `user_api_key_auth.py` | 核心：验证 API Key，从 Redis/PG 获取权限信息 |
| `handle_jwt.py` | JWT Token 解析与验证 |
| `oauth2_check.py` | OAuth2 / OIDC 集成 |
| `auth_checks.py` | 速率、预算、团队权限多重检查 |
| `route_checks.py` | 按路由进行权限控制（哪些端点需要什么权限） |

**多租户数据模型（PostgreSQL via Prisma）：**
- `LiteLLM_VerificationToken` — API Key 表（含 spend、budget 字段）
- `LiteLLM_TeamTable` — Team/租户表
- `LiteLLM_UserTable` — 用户表
- `LiteLLM_SpendLogs` — 消费日志（异步批量写入）

### 3.6 策略引擎（proxy/policy_engine/）

新增的策略引擎，实现 **Guardrail 与请求上下文的灵活绑定**：

```yaml
# config.yaml 策略配置示例
policies:
  - name: "finance-team-policy"
    inherits: "base-policy"
    scope:
      teams: ["finance-*"]
      models: ["gpt-4*"]
    guardrails:
      add: ["pii-filter", "audit-log"]
      remove: ["content-filter"]
```

组件：`PolicyRegistry`（内存存储）→ `PolicyMatcher`（通配符匹配）→ `PolicyResolver`（继承链解析）→ 最终 Guardrail 列表

### 3.7 企业版功能（enterprise/enterprise_hooks/）

| Hook | 文件 | 功能 |
|------|------|------|
| 关键词封禁 | `banned_keywords.py` | 请求/响应关键词过滤 |
| 用户封禁 | `blocked_user_list.py` | 黑名单用户拦截 |
| OpenAI 内容审核 | `openai_moderation.py` | 调用 OpenAI Moderation API |
| Google 内容审核 | `google_text_moderation.py` | 调用 Google Safe Message API |
| Aporia AI 安全 | `aporia_ai.py` | 第三方 Guardrail 集成 |

### 3.8 自研模块：coworker（Spend Sync 独立进程）

这是项目最主要的自研扩展：

**背景问题：** LiteLLM 原生的 Spend 写入依赖 proxy_server 内部的 APScheduler，多实例部署时会重复写入同一笔账单。

**解决方案：**

```
Redis (BLPOP)              coworker进程（Leader Only）
maas:spend_logs  ──BLPOP──►  worker.py
maas:spend_update ──drain──► DB批量写入（PostgreSQL）
                             ↑
                    Redis SET NX PX（Leader Election）
                    coworker:leader:lock
                    coworker:leader:fencing（Epoch 防脑裂）
```

| 特性 | 实现 |
|------|------|
| Leader Election | Redis `SET NX PX`，TTL=15s，定期续约 |
| Fencing Token | `leader_epoch` 单调递增，防止旧 Leader 提交 |
| 高可用 | 多实例竞选，Leader 宕机后 15s 内重新选主 |
| 配置 | YAML（poll_interval、retry_times、blpop_timeout）|
| 监控 | GET /status 返回 is_leader、leader_epoch、last_heartbeat |

### 3.9 自研模块：vidaimock（压测 Mock 服务）

集成 [VidaiMock](https://github.com/vidaiUK/VidaiMock) 高性能 LLM 模拟服务，用于：
- 路由策略单元测试（不消耗真实 API 配额）
- 延迟策略（lowest_latency）的行为验证
- 熔断/降级场景的端到端压测

---

## 四、与 MaaS 平台设计的能力对比

| MaaS 平台需求 | routero-develop 是否覆盖 | 差距说明 |
|-------------|------------------------|---------|
| 协议适配（100+ 供应商） | ✅ 完全覆盖 | `llms/` 目录即可直接使用 |
| 语义缓存（Qdrant/Redis Vector） | ✅ 原生支持 | `QdrantSemanticCache` / `RedisSemanticCache` |
| 基础路由策略（延迟/成本/负载） | ✅ 覆盖 | 5种策略 + Cooldown 熔断 |
| API Key 管理 + 预算 | ✅ 基础覆盖 | Key/Team/User 三层，但无 Project 层 |
| 限流（TPM/RPM） | ✅ 覆盖 | Lua 滑动窗口，与 MaaS 设计一致 |
| SSE 流式代理 | ✅ 覆盖 | streaming_handler.py 统一处理 |
| 内容安全（Guardrails） | ✅ 企业版覆盖 | enterprise_hooks 支持 5 种检查 |
| Spend 持久化 | ✅ 自研增强（coworker）| 解决了多实例重复写入问题 |
| 路由策略生命周期（9状态机）| ❌ 不支持 | LiteLLM 无 draft/simulation/canary/rollback 状态 |
| 三层 19 角色 RBAC | ❌ 不支持 | 仅 Key/Team/User 3个维度，无 Project 层 |
| SAML2/OIDC SSO + SCIM | ⚠️ 部分支持 | 有 OAuth2/JWT，无 SCIM 2.0 自动同步 |
| 审计日志哈希链 | ❌ 不支持 | 仅简单 SpendLogs，无 SHA-256 链 |
| FinOps 四层预算体系 | ❌ 不支持 | 仅 Key/User/Team 三层，无 Platform 层 |
| 等保 2.0 / GDPR 合规报告 | ❌ 不支持 | 无合规报告模块 |
| 供应商合同价格管理 | ❌ 不支持 | 价格来自 model_prices_and_context_window.json 静态文件 |

---

## 五、关键设计亮点（值得借鉴）

### 5.1 双级缓存（DualCache）
```
请求 → 本地 InMemoryCache（微秒级） → 未命中 → Redis（毫秒级）
```
避免所有请求都打到 Redis，高频 Key（如热门模型列表）完全在内存中命中。

### 5.2 批量 Redis 写入（BaseRoutingStrategy）
路由策略更新 TPM/RPM 计数时，使用 `RedisPipelineIncrementOperation` 队列攒批，`DEFAULT_REDIS_SYNC_INTERVAL` 周期统一 Pipeline 提交，大幅减少 Redis 连接开销。

### 5.3 供应商翻译层解耦（BaseConfig 模式）
每个供应商实现 `transform_request()` / `transform_response()`，`BaseLLMHTTPHandler` 统一调用，新增供应商只需新建一个文件，不改主流程。MaaS 平台 adapter-service 应沿用此模式。

### 5.4 语义路由（AutoRouter）
使用 `semantic-router` 库，根据 Prompt 向量相似度选择最合适的模型（如：技术类问题路由 claude，创意类路由 gpt-4o）。这比纯基于性能指标的路由更智能，可用于 MaaS 平台未来的智能路由扩展。

### 5.5 coworker Leader Election 设计
多实例 Spend 写入通过 Redis 分布式锁保证单主，Fencing Token（leader_epoch）防止网络分区场景下旧 Leader 的写入导致数据错误。MaaS platform 的 billing-service 可参考此方案处理高并发计费落库场景。

---

## 六、选型建议

### 方案 A：直接基于 routero-develop 扩展（推荐）

在该 fork 基础上继续开发，在 `enterprise_hooks/` 或 `proxy/hooks/` 中实现 MaaS 差异化能力：

| 需要补充的能力 | 实现位置 | 工作量 |
|-------------|---------|-------|
| Project 层预算 | `proxy/hooks/max_budget_limiter.py` 扩展 | 小 |
| SCIM 2.0 同步 | `proxy/management_endpoints/` 新增 | 中 |
| 路由策略 9 状态生命周期 | `proxy/management_endpoints/` + DB Schema | 大 |
| 审计哈希链 | `proxy/hooks/` 新增 CustomLogger | 中 |
| 合规报告 | 独立微服务（compliance-service），不在 LiteLLM 内 | 大 |

**优势：** 100+ 供应商适配、语义缓存、路由算法直接复用，节省约 60% 核心开发工作量。  
**劣势：** Python 为主语言，与 MaaS 的 Go 主技术栈存在语言边界，需通过 gRPC 桥接。

### 方案 B：LiteLLM 作为 adapter-service 的 Python Sidecar（当前设计）

仅将 LiteLLM 用于协议翻译，其余服务（auth、billing、compliance 等）全部自研 Go 实现。已在 `04-adapter-service详细设计.md` V2.1 中体现。

**优势：** Go 主栈统一，自研部分完全可控。  
**劣势：** 路由、限流、缓存等能力需重复造轮子。

### 推荐

**优先采用方案 A**，以 routero-develop 为基础网关，自研 compliance-service、增强版 auth-service（SCIM/等保），通过 Kafka + gRPC 解耦。这样可以在 6 个月内交付全功能平台，而方案 B 至少需要 12 个月。

---

## 七、文件索引（走读覆盖的关键文件）

| 文件 | 重要程度 | 说明 |
|------|---------|------|
| `ARCHITECTURE.md` | ⭐⭐⭐⭐⭐ | 官方架构文档，必读 |
| `litellm/proxy/proxy_server.py` | ⭐⭐⭐⭐⭐ | 主入口，所有 API 端点注册 |
| `litellm/router.py` | ⭐⭐⭐⭐⭐ | 路由器核心，负载均衡逻辑 |
| `litellm/proxy/auth/user_api_key_auth.py` | ⭐⭐⭐⭐⭐ | 认证核心 |
| `litellm/router_strategy/lowest_latency.py` | ⭐⭐⭐⭐ | 最低延迟路由算法 |
| `litellm/router_utils/cooldown_handlers.py` | ⭐⭐⭐⭐ | 熔断降级 |
| `litellm/caching/qdrant_semantic_cache.py` | ⭐⭐⭐⭐ | 语义缓存（Qdrant）|
| `litellm/caching/dual_cache.py` | ⭐⭐⭐⭐ | 双级缓存设计 |
| `litellm/proxy/hooks/parallel_request_limiter_v3.py` | ⭐⭐⭐⭐ | 限流（Lua脚本）|
| `litellm/proxy/policy_engine/` | ⭐⭐⭐ | 策略引擎（Guardrail 绑定）|
| `litellm/proxy/db/db_spend_update_writer.py` | ⭐⭐⭐ | 计费写入优化 |
| `coworker/src/coworker/worker.py` | ⭐⭐⭐⭐ | 自研：Spend 同步 + Leader Election |
| `litellm/router_strategy/auto_router/auto_router.py` | ⭐⭐⭐ | 语义路由（semantic-router）|
| `litellm/llms/anthropic/chat/transformation.py` | ⭐⭐⭐ | Anthropic 协议转换示例 |
| `litellm/llms/dashscope/` | ⭐⭐⭐ | 阿里云百炼协议转换 |
