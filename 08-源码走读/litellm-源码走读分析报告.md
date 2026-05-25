# LiteLLM 源码走读分析报告

**走读日期：** 2026年05月25日  
**源码仓库：** [BerriAI/litellm](https://github.com/BerriAI/litellm) (main branch)  
**代码规模：** Python 84.2% + TypeScript 14.1%，核心 SDK ~600KB  
**开源协议：** MIT License  
**对标价值：** ★★★★★ (MaaS adapter-service 最直接参考对象)

---

## 一、项目定性

LiteLLM 是一款成熟的开源 LLM Gateway，提供两种使用形态：

| 形态 | 入口 | 适用场景 |
|------|------|---------|
| **Python SDK** | `from litellm import completion` | 应用内嵌调用，自己管路由 |
| **Proxy Server** | `litellm --port 4000` | 独立部署网关，OpenAI 兼容入口 |

MaaS 只使用了 LiteLLM 的 **SDK 形态**（`litellm.acompletion()`）作为协议翻译器，并自研了 Router、KeyPool、CircuitBreaker、RetryEngine 等替代其 Proxy/Router 功能。走读的重点是理解 LiteLLM 在这些自研对等模块上的设计差异。

---

## 二、整体架构

```
┌──────────────────────────────────────────────────────────────┐
│                    LiteLLM 源码架构                            │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌─────────────────┐  ┌─────────────────┐                    │
│  │   SDK 入口       │  │  Proxy 入口      │                    │
│  │  main.py         │  │  proxy/           │                    │
│  │  completion()    │  │  proxy_server.py  │                    │
│  │  acompletion()   │  │  (FastAPI, 15K行) │                    │
│  └───────┬─────────┘  └────────┬────────┘                    │
│          │                      │                             │
│          └──────────┬───────────┘                             │
│                     ▼                                         │
│  ┌─────────────────────────────────────┐                     │
│  │  router.py (~11,750 行)             │                     │
│  │  Router 类: 负载均衡 / 故障转移      │                     │
│  │  Cooldown / Health / Fallback 链    │                     │
│  │  Pre-Call Checks / Callbacks        │                     │
│  └──────────────┬──────────────────────┘                     │
│                 ▼                                             │
│  ┌─────────────────────────────────────┐                     │
│  │  llms/  100+ 供应商适配器            │                     │
│  │  openai.py / anthropic.py / ...     │                     │
│  │  统一入口: 所有适配器接收 OpenAI 格式  │                     │
│  │  转换为各供应商原生 HTTP 请求         │                     │
│  └─────────────────────────────────────┘                     │
│                                                              │
│  支撑层:                                                      │
│  ├── caching/          (Redis + InMemory 双层缓存)            │
│  ├── router_strategy/  (5种路由策略处理器)                    │
│  ├── proxy/            (Proxy Server: 认证/限流/虚拟Key/预算) │
│  ├── secret_managers/  (凭证管理)                             │
│  └── integrations/     (日志/监控回调)                        │
│                                                              │
│  UI: ui/litellm-dashboard/  (React + TypeScript 管理面板)     │
└──────────────────────────────────────────────────────────────┘
```

---

## 三、核心模块逐层走读

### 3.1 SDK 入口层 (`main.py` + `__init__.py`)

**文件:** `litellm/main.py`, `litellm/__init__.py`

SDK 提供两个核心调用路径：

```python
# 路径1: 直接调用 (不经过 Router)
from litellm import completion
response = completion(model="openai/gpt-4o", messages=[...])

# 路径2: 经过 Router (负载均衡/故障转移)
from litellm import Router
router = Router(model_list=[...])
response = await router.acompletion(model="gpt-4o", messages=[...])
```

**Provider 路由机制:** `completion()` 的 `model` 参数使用 `provider/model` 格式（如 `openai/gpt-4o`），LiteLLM 内部通过 `get_llm_provider()` 解析出 provider 名称和模型名，然后调用 `llms/{provider}.py` 中的对应实现。

**关键设计:** 所有厂商适配器统一接收 OpenAI 格式的 `messages` 参数，在内部转换为各厂商的原生请求格式。这个设计保证了 API 的稳定性和厂商可替换性。

### 3.2 Router 层 (`router.py`, ~11,750行)

**文件:** `litellm/router.py`

这是 LiteLLM 最核心也最复杂的模块。`Router` 类的 `__init__` 接收 ~60 个参数：

#### 3.2.1 路由数据模型

```python
# 部署列表 (model_list 参数)
model_list = [
    {
        "model_name": "gpt-4o",           # 别名
        "litellm_params": {
            "model": "openai/gpt-4o",     # 实际提供商模型
            "api_key": "sk-xxx",
            "api_base": "https://...",
        }
    },
    # 同一逻辑模型下面的多个实例
    {
        "model_name": "gpt-4o",
        "litellm_params": {
            "model": "azure/gpt-4o",
            "api_key": "...",
            "api_base": "https://xxx.openai.azure.com",
        }
    },
]
```

Router 在 `set_model_list()` 中构建三个 O(1) 查找 Map：

| Map | Key | 用途 |
|-----|-----|------|
| `model_name_to_deployment_indices` | model_alias | 同一逻辑模型的所有部署实例 |
| `model_id_to_deployment_index` | deployment_id | 按 ID 定位具体部署 |
| `team_model_to_deployment_indices` | (team_id, model_name) | 按租户+模型定位 |

#### 3.2.2 五种路由策略

```python
# 源码实现在 litellm/router_strategy/ 下
routing_strategy:
  "simple-shuffle"          → simple_shuffle()        (加权随机)
  "least-busy"              → LeastBusyLoggingHandler  (最少并发)
  "usage-based-routing"     → LowestTPMLoggingHandler  (最低TPM用量)
  "usage-based-routing-v2"  → LowestTPMLoggingHandler_v2
  "latency-based-routing"   → LowestLatencyLoggingHandler (最低延迟)
  "cost-based-routing"      → LowestCostLoggingHandler    (最低成本)
```

每个策略是一个独立的 handler 类，实现统一接口。`routing_strategy_init()` 在 Router 初始化时根据配置装配。

**与 MaaS 差异:**
- LiteLLM: 策略是 Router 对象的一个配置参数，单维选择
- MaaS: routing-service 是独立微服务，四维加权评分 `Score = W1×cost + W2×latency + W3×capability + W4×health`，策略是独立的数据对象（30+字段）

#### 3.2.3 故障处理三重机制

```
Cooldown（冷却）:
  ├── 触发条件: 连续 allow_fails 次失败 (默认3次)
  ├── 冷却时间: cooldown_time (默认30s)
  ├── 实现: CooldownCache → DualCache (Redis + InMemory)
  └── 状态: cooldown_list 记录当前冷却的 deployment

Health Check（健康检查）:
  ├── DeploymentHealthCache 管理每个 deployment 的健康状态
  ├── staleness_threshold = HEALTH_CHECK_INTERVAL × STALENESS_MULTIPLIER
  └── 独立于 cooldown，可分别启用

Fallback（故障转移）:
  ├── 三层: 通用 fallback / context_window_fallback / content_policy_fallback
  ├── max_fallbacks 限制链长度
  └── "*" 通配可应用于所有模型
```

**与 MaaS 差异:**
- LiteLLM: Cooldown 是基于失败计数的简单机制，在 Router 进程内
- MaaS: CircuitBreaker 是完整三状态机 (CLOSED→OPEN→HALF_OPEN)，在 adapter-service 内独立管理，支持 HALF_OPEN 探测

#### 3.2.4 回调驱动的指标收集

```python
# Router.__init__ 中注册三个全局回调
litellm.success_callback.append(deployment_callback_on_success)
litellm.failure_callback.append(deployment_callback_on_failure)

# 成功回调: 记录每分钟成功数
# 失败回调: 增加 failure 计数 → 触发 cooldown
```

所有回调数据存储在 `failed_calls` (InMemoryCache, 1分钟窗口)，Cooldown 逻辑据此做决策。

### 3.3 Proxy Server 层 (`proxy/proxy_server.py`, ~15,000行)

**技术栈:** FastAPI + Prisma (ORM) + PostgreSQL

**核心请求流程:**

```
HTTP Request → FastAPI 路由 → 认证中间件
  → Virtual Key 解析 (虚拟Key→真实Key映射)
  → 预算检查 (team/customer/end-user 三级)
  → 限流检查 (RPM/TPM)
  → Guardrails 检查 (内容安全)
  → Router.acompletion() (路由+调用)
  → 日志埋点 (Spend Logs + Prometheus)
  → HTTP Response
```

**Proxy 独有功能 (SDK 形态不提供):**
- Virtual Keys: 虚拟 Key 映射到真实 Provider Key，隐藏供应商凭证
- Spend Tracking: 按 team/customer/end-user 追踪成本
- Budget Management: 分层的预算上限 + 告警
- Rate Limiting: RPM/TPM 限流
- Admin Dashboard: React UI 管理面板

### 3.4 Provider 适配器层 (`llms/`)

每个 Provider 一个 Python 文件（如 `openai.py`, `anthropic.py`, `gemini.py`），实现统一接口：

```python
# 每个 provider 导出的核心函数
def completion(
    model: str,
    messages: List[Dict],
    api_key: str,
    api_base: str,
    stream: bool,
    temperature: float,
    max_tokens: int,
    **kwargs,
) -> ModelResponse:
    # 1. 将 OpenAI 格式的消息转换为目标供应商格式
    # 2. 构建 HTTP 请求
    # 3. 调用供应商 API
    # 4. 将响应统一为 ModelResponse 格式
    pass
```

**关键设计模式:** 所有 Provider 输出统一为 `ModelResponse` 格式（内含 `choices[].message.content` 和 `usage`），保证上层 Router/Proxy 无需感知底层差异。

### 3.5 支撑模块

| 模块 | 功能 |
|------|------|
| `caching/` | DualCache (InMemory + Redis)，用于 cooldown 状态、TPM 计数、health 缓存 |
| `router_strategy/` | 5 种路由策略 handler，每种是一个独立类 |
| `secret_managers/` | 凭证管理 (AWS Secrets Manager, Vault 等) |
| `cost_calculator.py` | 按模型/Token 计算成本 |
| `budget_manager.py` | 预算管理 |
| `exceptions.py` | 统一的异常体系 (RateLimitError, Timeout, AuthenticationError 等 9 种) |

---

## 四、关键设计决策与权衡

### 4.1 Router 与 Proxy 的耦合

LiteLLM 的 Router 和 Proxy 在同一个 Python 进程内紧密耦合。`proxy_server.py` 直接 import `Router` 并实例化。

**优点:** 零 IPC 开销，部署简单（一个进程搞定）
**缺点:** Router 无法独立扩展，Proxy 负载高时 Router 也会受影响

**MaaS 的设计:** routing-service (Go) 是独立微服务，通过 gRPC 与 gateway-service 通信。追求独立扩展和故障隔离。

### 4.2 路由策略 vs 路由服务

LiteLLM 的路由是 **进程内**的配置驱动行为。Routing Strategy 是 Router 对象的一个枚举参数。没有"策略即数据"的概念——策略不能被外部系统创建、查询、审批、版本化。

**MaaS 的做法:** routing_policy 是独立的数据表（30+字段），有完整的 CRUD + 审批工作流 + 版本管理 + 模拟引擎。策略是"一等公民"。

### 4.3 故障处理的粒度

LiteLLM 的 Cooldown 是基于 `allowed_fails` 阈值的时间窗口冷却。没有 HALF_OPEN 探测状态——冷却期满后直接恢复，不经过探测阶段。

**MaaS 的做法:** CircuitBreaker 是完整的三状态机 (CLOSED→OPEN→HALF_OPEN→CLOSED)，HALF_OPEN 状态下只允许 3 个探测请求，成功率达标才恢复。更保守但更安全。

### 4.4 键与预算的模型

LiteLLM 的 Virtual Key 是字典映射：虚拟 Key → 真实 Provider Key + budget 限制。预算是附加在 Key 上的简单计数器。

**MaaS 的做法:** 四层预算体系 (Platform → Tenant → Project → Key) 是独立的数据模型，有独立的配置、审批、告警策略。ApiKey 有 5 种类型 (mk-prod/mk-dev/mk-ci/mk-pg/mk-sub)，各有独立的生命周期。

---

## 五、MaaS 与 LiteLLM 的定位对比

| 维度 | LiteLLM | MaaS |
|------|---------|------|
| **形态** | Python 单体 (SDK + Proxy) | 10 微服务 (Go + Python) |
| **路由** | 进程内 Router，5 种策略 | 独立 routing-service，四维加权 + 策略即数据 |
| **故障处理** | Cooldown + Fallback | CircuitBreaker (三状态) + Fallback链 (L1-L5) |
| **接入** | 仅 OpenAI 兼容端点 | OpenAI + Anthropic + Gemini + WebSocket |
| **协议适配** | 自研 llms/ 适配器 | 复用 LiteLLM SDK (acompletion)，仅做协议翻译 |
| **多租户** | Virtual Keys + Teams | 完整 19 角色 RBAC + 审批工作流 + SSO/SCIM |
| **合规** | 无原生支持 | 独立 compliance-service (Guardrails/KMS/ZDR/审计链) |
| **观测** | Prometheus + Spend Logs | 独立 llmops-trace-service (44字段 Trace + Session + ClickHouse) |
| **评测** | 无 | 独立 prompt-eval-service (A/B实验+LLM-as-Judge+质量门禁) |
| **运营** | Admin Dashboard | 运营后台 + 开发者 Console + FinOps 仪表板 |
| **部署** | 1 进程 / 2 进程 | K8s 10 微服务，HPA 弹性伸缩 |
| **适用** | 技术团队自建 AI 基础设施 | 企业级 AI 运营平台 (多租户 SaaS + 私有化) |

---

## 六、从 LiteLLM 学到什么

### ✅ 值得借鉴的设计

1. **Provider 适配器模式**: `provider/model` 字符串驱动分发 + 统一 `ModelResponse`，模块化程度高，新增供应商只需添加一个 Python 文件
2. **DualCache 架构**: InMemory + Redis 双层缓存，兼顾性能和水平扩展，在 adapter-service 的 KeyPool 缓存设计中可以直接参考
3. **回调驱动的指标收集**: 成功/失败回调自动更新 Cooldown 状态，解耦了调用逻辑和健康管理
4. **Virtual Keys 设计**: 虚拟 Key 映射真实 Key 的抽象模式，MaaS 的 Sub Key (mk-sub-) 功能设计可参考其 key_type 层级继承逻辑
5. **预调用检查链**: `PreCallChecks` 的可插拔过滤器模式（PromptCaching / DeploymentAffinity / RateLimiting），可参考引入 MaaS 网关层

### ⚠️ 需要注意的局限

1. **Router 15K 行单文件**: 复杂度过高，维护困难。MaaS 将路由拆为独立服务 + 独立数据模型是更健康的架构
2. **Proxy 15K 行单文件**: 同样的问题。MaaS 的 11 步中间件链更模块化
3. **Cooldown 机制过于简单**: 无 HALF_OPEN 探测，故障恢复可能二次雪崩。MaaS 的完整熔断器更安全
4. **无数据分级/合规**: 完全没有 L0-L4 数据分级、地域控制、ZDR 等概念。企业级客户需要大量二次开发
5. **无评测体系**: 完全没有 Prompt 版本管理、A/B 实验、质量门禁等能力
6. **无策略即代码**: 路由策略是代码配置而非数据对象，无法做审批、版本、模拟

---

## 七、文件导航速查

| LiteLLM 文件/目录 | 行数 | 职责 | MaaS 对应 |
|-------------------|------|------|----------|
| `router.py` | ~11,750 | Router 负载均衡/故障转移 | routing-service (独立Go服务) |
| `proxy/proxy_server.py` | ~15,000 | Proxy 网关 | gateway-service (独立Go服务) |
| `llms/openai.py` | ~2,000 | OpenAI 适配器 | adapter-service 内的 LiteLLM 调用 |
| `llms/anthropic.py` | ~1,500 | Anthropic 适配器 | 同上 |
| `caching/` | ~2,000 | 双层缓存 | adapter-service 的 Redis Cache |
| `router_strategy/` | ~1,500 | 路由策略实现 | routing-service 的评分引擎 |
| `cost_calculator.py` | ~3,000 | 成本计算 | billing-service 的计量引擎 |
| `budget_manager.py` | ~1,000 | 预算管理 | billing-service 的四层预算 |
| `proxy/auth/` | ~2,000 | 认证授权 | auth-service (19角色RBAC) |
| `ui/litellm-dashboard/` | TypeScript | Admin UI | Console + Admin 两个独立前端 |

---

*本文档基于 LiteLLM main 分支静态分析编写，代码引用截至 2026年05月25日。LiteLLM 迭代极快，具体实现可能已变化，设计模式通常保持稳定。*
