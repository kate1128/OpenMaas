# MaaS平台 API接口设计文档

**文档版本：** V2.0  
**编写日期：** 2026年05月25日  
**文档状态：** 初稿  
**关联PRD：** `产品设计/MaaS-PRD-V2.0/`  
**API Base URL：** `https://api.maas-platform.com`  
**密级：** 内部

**变更说明：** V2.0 对齐 PRD V2.0：新增 Anthropic / Gemini 兼容端点；认证升级为 API Key（5种类型）+ SSO JWT 双模；新增 RBAC 权限级别（19角色）；新增合规安全 API（Guardrails/KMS/审计）；新增 FinOps API（成本仪表板/异常检测/节省建议）；新增 Prompt评测 API（A/B实验/评测任务）；错误码扩展。

---

## 目录

1. [接口设计规范](#接口设计规范)
2. [认证与授权](#认证与授权)
3. [推理接口（OpenAI兼容）](#推理接口openai兼容)
4. [模型管理接口](#模型管理接口)
5. [API密钥管理接口](#api密钥管理接口)
6. [路由策略管理接口](#路由策略管理接口)
7. [计量计费接口](#计量计费接口)
8. [监控与统计接口](#监控与统计接口)
9. [错误码规范](#错误码规范)
10. [限流说明](#限流说明)

---

## 1. 接口设计规范

### 1.1 URL结构

```
https://api.maas-platform.com/{version}/{resource}/{action}
```

- **version**：API版本，当前为 `v1`
- **resource**：资源名称（复数名词）
- **action**：可选的动作名称（CRUD以外的操作）

### 1.2 HTTP方法约定

| 方法 | 操作 | 幂等性 |
|------|------|--------|
| GET | 查询资源 | 是 |
| POST | 创建资源或发起操作 | 否 |
| PUT | 全量更新资源 | 是 |
| PATCH | 局部更新资源 | 是 |
| DELETE | 删除资源 | 是 |

### 1.3 响应格式

**成功响应：**
```json
{
  "object": "resource_type",
  "data": { /* 资源内容 */ },
  "request_id": "req_abcdef123456"
}
```

**列表响应：**
```json
{
  "object": "list",
  "data": [ /* 资源数组 */ ],
  "total": 100,
  "page": 1,
  "page_size": 20,
  "request_id": "req_abcdef123456"
}
```

**错误响应：**
```json
{
  "error": {
    "code": "insufficient_quota",
    "message": "API Key额度不足，当前剩余 1000 tokens",
    "type": "quota_error",
    "param": null
  },
  "request_id": "req_abcdef123456"
}
```

### 1.4 通用请求头

| Header | 必选 | 说明 |
|--------|------|------|
| `Authorization: Bearer {api_key}` | 条件必选 | API Key认证（5种类型：mk-prod/mk-dev/mk-ci/mk-pg/mk-sub） |
| `X-MaaS-Token: {jwt_token}` | 条件必选 | SSO JWT认证（SAML2/OIDC），替代API Key |
| `Content-Type: application/json` | 是（POST/PUT/PATCH） | 请求体格式 |
| `X-Request-ID` | 否 | 客户端请求ID，用于幂等与追踪 |
| `X-Tenant-ID` | 否 | 多租户场景下指定租户（管理员接口） |

---

## 2. 认证与授权

### 2.1 双模认证

**模式一：API Key 认证**
- Header：`Authorization: Bearer {api_key}`
- Key格式：`mk-prod-{32位}` / `mk-dev-{32位}` / `mk-ci-{32位}` / `mk-pg-{32位}` / `mk-sub-{32位}`
- 认证方式：HMAC-SHA256(key, platform_secret) → Redis/DB查询
- Key生命周期：`ACTIVE → SUSPENDED/EXPIRED/REVOKED → ...`

**模式二：SSO JWT 认证**
- Header：`X-MaaS-Token: {jwt_token}`
- gRPC auth-service.ValidateJWT：验证 iss/exp/aud/sig
- 解析 claims：{tenant_id, user_id, roles[]}

### 2.2 权限级别（19角色RBAC — PRD §01）

| 角色层级 | 角色ID | 说明 |
|---------|--------|------|
| **平台级（8个）** | `platform_owner`, `platform_admin`, `security_officer`, `auditor`, `billing_admin`, `support_engineer`, `readonly_viewer` | 平台级管理 |
| **租户级（6个）** | `tenant_admin`, `billing_admin`, `security_officer`, `developer_admin`, `developer`, `procurement_officer` | 租户级管理 |
| **项目级（5个）** | `project_admin`, `project_developer`, `project_viewer`, `project_model_curator`, `project_eval_engineer` | 项目级协作 |

| 接口分组 | 所需权限（示例） | 说明 |
|---------|---------|------|
| 推理接口 | `api_key:invoke` | 基础开发者权限 |
| 模型查询 | `model:read` | 基础开发者权限 |
| API Key管理 | `apikey:manage` | 开发者自管理 |
| 路由策略 | `routing:manage` | 项目管理员 |
| 计费查询 | `billing:read` | 开发者/管理员 |
| 合规安全策略 | `compliance:manage` | 安全负责人 |
| 平台管理 | `platform:admin` | 平台管理员 |

---

## 3. 推理接口（OpenAI兼容）

> 以下接口完全兼容 OpenAI API 格式，已有 OpenAI SDK 的项目仅需修改 `base_url` 即可迁移。

### 3.1 对话补全

**接口：** `POST /v1/chat/completions`

**请求体：**
```json
{
  "model": "gpt-4o",
  "messages": [
    {
      "role": "system",
      "content": "You are a helpful assistant."
    },
    {
      "role": "user",
      "content": "你好，介绍一下MaaS平台"
    }
  ],
  "temperature": 0.7,
  "max_tokens": 1024,
  "stream": false,
  "top_p": 1.0,
  "n": 1,
  "stop": null,
  "presence_penalty": 0,
  "frequency_penalty": 0,
  "user": "user_identifier_optional"
}
```

**请求参数说明：**

| 参数 | 类型 | 必选 | 默认值 | 说明 |
|------|------|------|--------|------|
| `model` | string | 是 | - | 模型标识符，支持平台内所有已接入模型 |
| `messages` | array | 是 | - | 对话消息列表 |
| `temperature` | float | 否 | 1.0 | 采样温度，0-2，越高越随机 |
| `max_tokens` | integer | 否 | null | 最大生成Token数，null表示不限制 |
| `stream` | boolean | 否 | false | 是否使用SSE流式返回 |
| `top_p` | float | 否 | 1.0 | 核采样概率，0-1 |
| `n` | integer | 否 | 1 | 生成结果数量 |
| `stop` | string\|array | 否 | null | 停止词 |

**平台扩展参数（非OpenAI标准，可选）：**

| 参数 | 类型 | 说明 |
|------|------|------|
| `x_routing_strategy` | string | 强制指定路由策略：`cost_first`/`performance_first`/`specified` |
| `x_disable_cache` | boolean | 是否跳过语义缓存，默认false |
| `x_fallback_models` | array | 本次请求的临时备用模型列表 |

**成功响应（非流式）：**
```json
{
  "id": "chatcmpl-9aBcDeFgHiJkLmNo",
  "object": "chat.completion",
  "created": 1747555200,
  "model": "gpt-4o",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "MaaS平台是一个企业级大模型即服务平台..."
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 45,
    "completion_tokens": 128,
    "cached_input_tokens": 0,
    "total_tokens": 173
  },
  "x_maas": {
    "request_id": "req_abcdef123456",
    "routed_to": "openai/gpt-4o",
    "cache_hit": false,
    "latency_ms": 342
  }
}
```

**流式响应（stream=true）：** 使用 Server-Sent Events (SSE)，每个chunk格式：
```
data: {"id":"chatcmpl-xxx","object":"chat.completion.chunk","choices":[{"delta":{"content":"MaaS"},"index":0}]}

data: [DONE]
```

---

### 3.2 文本嵌入

**接口：** `POST /v1/embeddings`

**请求体：**
```json
{
  "input": "需要向量化的文本",
  "model": "text-embedding-3-small",
  "encoding_format": "float"
}
```

**响应：**
```json
{
  "object": "list",
  "data": [
    {
      "object": "embedding",
      "index": 0,
      "embedding": [0.0023064255, -0.009327292, ...]
    }
  ],
  "model": "text-embedding-3-small",
  "usage": {
    "prompt_tokens": 8,
    "total_tokens": 8
  }
}
```

---

### 3.3 查询可用模型列表

**接口：** `GET /v1/models`

**Query参数：**

| 参数 | 类型 | 说明 |
|------|------|------|
| `type` | string | 过滤模型类型：text/embedding/multimodal |
| `provider` | string | 过滤供应商：openai/anthropic/qwen 等 |
| `capability` | string | 过滤能力标签：code/chat/rag 等 |

**响应：**
```json
{
  "object": "list",
  "data": [
    {
      "id": "gpt-4o",
      "object": "model",
      "created": 1715367049,
      "owned_by": "openai",
      "x_maas": {
        "display_name": "GPT-4o",
        "model_type": "text",
        "context_length": 128000,
        "capability_tags": ["chat", "code", "vision"],
        "input_price_per_1k_tokens": 0.005,
        "output_price_per_1k_tokens": 0.015,
        "status": "active"
      }
    }
  ]
}
```

---

## 3A. Anthropic 兼容端点（V2.0 新增）

### 3A.1 Anthropic Messages

**接口：** `POST /v1/anthropic/messages`

完全兼容 Anthropic Messages API。请求体格式参考 [Anthropic API 文档](https://docs.anthropic.com/en/api/messages)。

**兼容性说明：** gateway 自动将 Anthropic 格式归一化为 StandardRequest，进入标准中间件链（认证/限流/合规/路由），adapter 侧 LiteLLM 自动转换为 Anthropic 原生协议调用。

### 3A.2 Anthropic 流式 Messages

**接口：** `POST /v1/anthropic/messages/stream`

SSE 流式响应，兼容 Anthropic Streaming 格式。

---

## 3B. Gemini 兼容端点（V2.0 新增）

### 3B.1 Gemini 内容生成

**接口：** `POST /v1/gemini/generateContent`

兼容 Google Gemini API 格式。

### 3B.2 Gemini 流式生成

**接口：** `POST /v1/gemini/streamGenerateContent`

SSE 流式响应。

---

## 3C. WebSocket 端点（V2.0 新增）

**接口：** `ws://api.maas-platform.com/v1/ws`

基于 WebSocket 的流式对话代理。连接后发送 JSON 消息（包含 model / messages / stream=true），接收流式 text/event-stream 帧。

---

## 4. 模型管理接口

> 以下接口需要控制台登录认证（Bearer Token），非 API Key。

### 4.1 获取模型列表（管理视图）

**接口：** `GET /v1/platform/models`

**Query参数：**

| 参数 | 类型 | 说明 |
|------|------|------|
| `page` | integer | 页码，默认1 |
| `page_size` | integer | 每页条数，默认20，最大100 |
| `status` | string | 过滤状态 |
| `is_public` | boolean | 是否公共模型 |

---

### 4.2 注册新模型

**接口：** `POST /v1/platform/models`  
**权限：** `admin:model:create`

**请求体：**
```json
{
  "name": "qwen-max",
  "display_name": "通义千问-Max",
  "provider": "qwen",
  "model_type": "text",
  "capability_tags": ["chat", "code"],
  "context_length": 32768,
  "input_price": 0.04,
  "output_price": 0.12,
  "is_public": true,
  "metadata": {
    "description": "通义千问旗舰模型，支持复杂推理",
    "homepage_url": "https://qwenlm.github.io/"
  }
}
```

**响应：** `201 Created`，返回创建的模型对象。

---

### 4.3 创建精调任务

**接口：** `POST /v1/finetune/jobs`  
**权限：** `model:finetune`

**请求体：**
```json
{
  "base_model": "qwen-7b",
  "output_model_name": "my-custom-assistant-v1",
  "method": "lora",
  "training_file": "file-abc123",
  "validation_file": "file-def456",
  "hyperparameters": {
    "n_epochs": 3,
    "learning_rate_multiplier": 0.1,
    "batch_size": 8,
    "lora_r": 16,
    "lora_alpha": 32
  },
  "gpu_resource": {
    "gpu_type": "A100",
    "gpu_count": 2
  }
}
```

**响应：** `201 Created`
```json
{
  "id": "ftjob-abc123",
  "object": "fine_tuning.job",
  "status": "pending",
  "model": "qwen-7b",
  "fine_tuned_model": null,
  "created_at": 1747555200,
  "estimated_start_at": 1747558800
}
```

---

### 4.4 查询精调任务状态

**接口：** `GET /v1/finetune/jobs/{job_id}`

**响应：**
```json
{
  "id": "ftjob-abc123",
  "object": "fine_tuning.job",
  "status": "running",
  "model": "qwen-7b",
  "fine_tuned_model": null,
  "created_at": 1747555200,
  "started_at": 1747558900,
  "training_metrics": {
    "step": 450,
    "total_steps": 900,
    "train_loss": 1.23,
    "train_acc": 0.78
  },
  "result_files": []
}
```

---

## 5. API密钥管理接口

### 5.1 创建API密钥

**接口：** `POST /v1/api-keys`

**请求体：**
```json
{
  "name": "生产环境密钥",
  "project_id": "proj-abc123",
  "token_quota": 10000000,
  "rate_limit_rpm": 100,
  "rate_limit_tpm": 100000,
  "ip_whitelist": ["192.168.1.0/24", "10.0.0.1"],
  "expires_at": "2027-01-01T00:00:00Z"
}
```

**响应：** `201 Created`  
> **注意：** `key` 字段仅在创建时返回一次，之后无法再查看完整密钥。

```json
{
  "id": "key-abc123",
  "object": "api_key",
  "name": "生产环境密钥",
  "key": "sk-maas-AbCdEfGhIjKlMnOpQrStUvWxYz012345",
  "key_prefix": "sk-maas-AbCd",
  "status": "active",
  "token_quota": 10000000,
  "token_used": 0,
  "created_at": "2026-05-14T08:00:00Z"
}
```

---

### 5.2 查询API密钥列表

**接口：** `GET /v1/api-keys`

**Query参数：** `project_id`（可选）、`status`（可选）、`page`、`page_size`

---

### 5.3 更新API密钥

**接口：** `PATCH /v1/api-keys/{key_id}`

**请求体（部分更新）：**
```json
{
  "name": "新名称",
  "status": "disabled",
  "token_quota": 20000000
}
```

---

### 5.4 吊销API密钥

**接口：** `DELETE /v1/api-keys/{key_id}`

**响应：** `200 OK`
```json
{
  "id": "key-abc123",
  "object": "api_key",
  "status": "revoked",
  "revoked_at": "2026-05-14T10:00:00Z"
}
```

---

## 6. 路由策略管理接口

### 6.1 创建路由策略

**接口：** `POST /v1/routing/policies`  
**权限：** `routing:manage`

**请求体：**
```json
{
  "name": "成本优先策略-生产",
  "description": "生产环境API调用使用成本最优路由",
  "project_id": "proj-abc123",
  "strategy_type": "cost_first",
  "priority": 10,
  "match_conditions": {
    "api_key_ids": ["key-abc123"],
    "model_pattern": "*"
  },
  "target_models": ["endpoint-001", "endpoint-002"],
  "fallback_models": ["endpoint-003"],
  "weight_config": {
    "cost_weight": 0.6,
    "latency_weight": 0.25,
    "match_weight": 0.1,
    "stability_weight": 0.05
  }
}
```

---

### 6.2 策略模拟测试

**接口：** `POST /v1/routing/policies/{policy_id}/simulate`

**请求体：**
```json
{
  "model": "gpt-4o",
  "estimated_prompt_tokens": 500,
  "estimated_completion_tokens": 200
}
```

**响应：**
```json
{
  "selected_endpoint": "endpoint-001",
  "selected_model": "openai/gpt-4o",
  "predicted_cost": 0.00575,
  "predicted_latency_ms": 380,
  "candidate_scores": [
    {"endpoint_id": "endpoint-001", "score": 0.87, "reason": "成本最优"},
    {"endpoint_id": "endpoint-002", "score": 0.72, "reason": "备选"}
  ]
}
```

---

## 7. 计量计费接口

### 7.1 查询用量统计

**接口：** `GET /v1/billing/usage`

**Query参数：**

| 参数 | 类型 | 必选 | 说明 |
|------|------|------|------|
| `start_date` | string | 是 | 开始日期，格式 YYYY-MM-DD |
| `end_date` | string | 是 | 结束日期，格式 YYYY-MM-DD |
| `group_by` | string | 否 | 分组维度：model/project/api_key/day |
| `project_id` | string | 否 | 过滤指定项目 |
| `model` | string | 否 | 过滤指定模型 |

**响应：**
```json
{
  "object": "billing.usage_summary",
  "period": {
    "start": "2026-05-01",
    "end": "2026-05-14"
  },
  "total_tokens": 15623400,
  "total_cost": 234.56,
  "breakdown": [
    {
      "group": "gpt-4o",
      "prompt_tokens": 8200000,
      "completion_tokens": 3100000,
      "total_tokens": 11300000,
      "total_cost": 180.25
    },
    {
      "group": "qwen-max",
      "prompt_tokens": 3200000,
      "completion_tokens": 1123400,
      "total_tokens": 4323400,
      "total_cost": 54.31
    }
  ]
}
```

---

### 7.2 查询账单列表

**接口：** `GET /v1/billing/statements`

**Query参数：** `year`、`month`、`status`、`project_id`

---

### 7.3 查询预算配置

**接口：** `GET /v1/billing/budgets`

---

### 7.4 创建/更新预算配置

**接口：** `PUT /v1/billing/budgets/{budget_id}`

**请求体：**
```json
{
  "project_id": "proj-abc123",
  "period_type": "monthly",
  "cost_limit": 1000.00,
  "alert_threshold": 80.0,
  "notify_channels": ["email", "dingtalk"],
  "notify_users": ["user-001", "user-002"],
  "is_hard_limit": false
}
```

---

## 8. 监控与统计接口

### 8.1 获取平台实时指标

**接口：** `GET /v1/monitor/metrics`  
**权限：** `monitor:read`

**Query参数：** `metric_names`（逗号分隔）、`window`（时间窗口，如 5m/1h/24h）

**响应：**
```json
{
  "object": "monitor.metrics",
  "timestamp": "2026-05-14T08:00:00Z",
  "metrics": {
    "api_gateway.qps": 1250,
    "api_gateway.success_rate": 0.9992,
    "api_gateway.p95_latency_ms": 312,
    "cache.hit_rate": 0.23,
    "routing.strategy_hit_rate": 0.87,
    "gpu.cluster_utilization": 0.72
  }
}
```

---

### 8.2 查询模型实例健康状态

**接口：** `GET /v1/monitor/endpoints`

**响应：**
```json
{
  "object": "list",
  "data": [
    {
      "endpoint_id": "endpoint-001",
      "model": "gpt-4o",
      "health_status": "healthy",
      "last_latency_p95_ms": 310,
      "error_rate_1h": 0.001,
      "last_checked_at": "2026-05-14T07:59:30Z"
    }
  ]
}
```

---

## 9. 错误码规范

### 9.1 HTTP状态码

| 状态码 | 含义 |
|--------|------|
| 200 | 请求成功 |
| 201 | 资源创建成功 |
| 400 | 请求参数错误 |
| 401 | 未认证或认证失败 |
| 403 | 无权限执行此操作 |
| 404 | 资源不存在 |
| 429 | 请求频率超限 |
| 500 | 服务器内部错误 |
| 503 | 服务暂时不可用 |
| 504 | 上游模型服务超时 |

### 9.2 业务错误码（V2.0 扩展）

**通用/认证错误：**
| 错误码 | HTTP状态 | 说明 |
|--------|---------|------|
| `invalid_api_key` | 401 | API Key无效或已吊销 |
| `api_key_expired` | 401 | API Key已过期 |
| `api_key_suspended` | 401 | API Key已被暂停（Key泄露检测） |
| `jwt_validation_failed` | 401 | SSO JWT校验失败 |
| `permission_denied` | 403 | 无此操作权限（scope不足） |
| `ip_not_allowed` | 403 | 请求IP不在白名单 |
| `scope_denied` | 403 | 请求操作超出Key权限范围 |

**限流/资源错误：**
| 错误码 | HTTP状态 | 说明 |
|--------|---------|------|
| `rate_limit_exceeded` | 429 | 超出RPM/TPM限制 |
| `quota_exceeded` | 402 | Token额度不足（预检拒绝） |
| `budget_exceeded` | 402 | 超出预算上限（硬限制） |

**模型/路由错误：**
| 错误码 | HTTP状态 | 说明 |
|--------|---------|------|
| `model_not_found` | 404 | 指定模型不存在或未接入 |
| `model_deprecated` | 404 | 模型已弃用，请使用迁移目标模型 |
| `model_unavailable` | 503 | 模型当前无可用后端实例 |
| `context_length_exceeded` | 400 | 请求超过模型最大上下文长度 |

**合规安全错误（V2.0 新增）：**
| 错误码 | HTTP状态 | 说明 |
|--------|---------|------|
| `content_policy_violation` | 403 | 请求/响应内容违反安全策略 |
| `pii_detected` | 403 | PII检测命中（取决于策略配置） |
| `prompt_injection_detected` | 403 | Prompt注入攻击检测 |
| `region_restricted` | 451 | 数据驻留禁止路由到目标地域 |
| `zdr_limitation` | 403 | ZDR模式下不支持的操作 |

**上游错误：**
| 错误码 | HTTP状态 | 说明 |
|--------|---------|------|
| `upstream_timeout` | 504 | 上游模型API调用超时 |
| `upstream_unavailable` | 503 | 上游模型服务不可用 |
| `upstream_rate_limit` | 429 | 上游供应商限流 |
| `upstream_content_blocked` | 400 | 上游供应商内容拦截 |
| `upstream_connection_failed` | 502 | 上游连接失败（DNS/TCP/SSL） |
| `upstream_server_error` | 502 | 上游服务器内部错误 |
| `internal_server_error` | 500 | MaaS平台内部错误 |

---

## 10. 限流说明

### 10.1 默认限流配置

| 限流维度 | 免费套餐 | 专业套餐 | 企业套餐 |
|---------|---------|---------|---------|
| RPM（每分钟请求数） | 60 | 600 | 自定义 |
| TPM（每分钟Token数） | 100,000 | 2,000,000 | 自定义 |
| 单日Token上限 | 500,000 | 无限制 | 无限制 |

### 10.2 限流响应头

触发限流时，响应头会携带：

```
X-RateLimit-Limit-Requests: 600
X-RateLimit-Limit-Tokens: 2000000
X-RateLimit-Remaining-Requests: 0
X-RateLimit-Remaining-Tokens: 0
X-RateLimit-Reset-Requests: 2026-05-14T08:01:00Z
X-RateLimit-Reset-Tokens: 2026-05-14T08:01:00Z
Retry-After: 35
```

### 10.3 限流重试建议

客户端应实现指数退避重试策略：

```python
import time
import random

def call_with_retry(func, max_retries=3):
    for attempt in range(max_retries):
        try:
            return func()
        except RateLimitError as e:
            if attempt == max_retries - 1:
                raise
            wait = (2 ** attempt) + random.random()
            time.sleep(wait)
```

---

## 变更历史

| 版本 | 日期 | 修改内容 | 修改人 |
|------|------|---------|--------|
| V1.0 | 2026-05-14 | 初始版本，基于PRD V4.0生成 | — |
| V2.0 | 2026-05-25 | 对齐 PRD V2.0：新增 Anthropic/Gemini/WS 端点、19角色RBAC权限模型、5种API Key类型、SSO JWT双模认证、cached_tokens用量、合规安全错误码、区域限制（451）、注入检测（403） | — |
