# MaaS平台 产品文档（Product Docs）

> 本文档为面向外部开发者发布的官方产品文档，对应文档站 **docs.maas-platform.com**  
> 文档结构模仿 Stripe Docs / OpenAI Docs 的信息架构风格

**文档版本：** V1.0  
**编写日期：** 2026年05月14日  
**密级：** 对外公开

---

## 文档站结构

```
docs.maas-platform.com/
├── 快速开始 (Getting Started)
│   ├── 平台介绍
│   ├── 5分钟接入
│   └── 迁移指南（从 OpenAI 迁移）
├── 核心概念 (Concepts)
│   ├── 租户与项目
│   ├── API Key
│   ├── 智能路由
│   └── 语义缓存
├── API 参考 (API Reference)
│   ├── Chat Completions
│   ├── Embeddings
│   ├── Models
│   └── 错误码大全
├── 指南 (Guides)
│   ├── 流式输出
│   ├── 函数调用（Function Calling）
│   ├── 精调模型
│   └── 成本优化
├── SDK
│   ├── Python
│   ├── JavaScript/TypeScript
│   ├── Go
│   └── Java
└── 变更日志 (Changelog)
```

---

# 第一部分：快速开始

## 平台介绍

MaaS Platform 是一个**企业级大模型聚合服务平台**，提供统一的 OpenAI 兼容 API 接口，让开发者通过一套代码接入多家主流大模型服务商。

**主要特性：**

| 特性 | 说明 |
|------|------|
| **OpenAI 兼容** | 只需修改 `base_url`，现有 OpenAI 代码零改动迁移 |
| **多模型支持** | OpenAI、Anthropic、通义千问、文心一言、智谱 GLM、自托管模型 |
| **智能路由** | 自动在成本、延迟、质量间寻找最优平衡 |
| **语义缓存** | 相似请求复用结果，平均降低 30-50% Token 消耗 |
| **统一账单** | 一张账单看清所有模型消耗，按实际用量计费 |
| **安全合规** | API Key 哈希存储，支持 IP 白名单，操作全量审计 |

## 5分钟接入

### Step 1：获取 API Key

1. 登录 [控制台](https://console.maas-platform.com)
2. 进入「API Keys」→「新建 API Key」
3. 复制 API Key（**仅显示一次，请立即保存**）

### Step 2：发起第一个请求

```bash
curl https://api.maas-platform.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $MAAS_API_KEY" \
  -d '{
    "model": "qwen-turbo",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

### Step 3：使用 Python SDK

```python
pip install openai  # MaaS 完全兼容 OpenAI Python SDK
```

```python
from openai import OpenAI

client = OpenAI(
    api_key="YOUR_MAAS_API_KEY",
    base_url="https://api.maas-platform.com/v1"
)

response = client.chat.completions.create(
    model="qwen-turbo",
    messages=[{"role": "user", "content": "Hello!"}]
)
print(response.choices[0].message.content)
```

## 从 OpenAI 迁移

如果你已经在使用 OpenAI SDK，迁移只需两步：

**Before（使用 OpenAI）：**
```python
from openai import OpenAI
client = OpenAI(api_key="sk-openai-xxx")
```

**After（迁移到 MaaS）：**
```python
from openai import OpenAI
client = OpenAI(
    api_key="YOUR_MAAS_API_KEY",     # 改为 MaaS API Key
    base_url="https://api.maas-platform.com/v1"  # 添加 base_url
)
```

模型名称映射（如需使用相同模型名）：

| OpenAI 模型名 | MaaS 模型名 |
|-------------|------------|
| `gpt-4o` | `gpt-4o` |
| `gpt-4o-mini` | `gpt-4o-mini` |
| `gpt-3.5-turbo` | 建议迁移到 `qwen-turbo`（更便宜） |
| `text-embedding-3-small` | `text-embedding-3-small` |

---

# 第二部分：核心概念

## 租户与项目

```
租户（Tenant）
  = 一个企业 / 独立业务单元
  = 独立的计费单元
  = 独立的用量统计

项目（Project）
  = 租户下的业务分组
  = 不同环境隔离（dev / staging / prod）
  = 可单独设置预算上限

API Key
  = 绑定到具体项目的调用凭证
  = 携带速率限制、IP 白名单等配置
  = 一个项目可有多个 Key
```

## API Key 安全

- **存储方式**：MaaS 只保存 SHA-256 哈希，明文不落盘
- **传输方式**：通过 Authorization Header 传输，全链路 TLS 1.3 加密
- **最佳实践**：用环境变量存储，生产环境设置 IP 白名单

```bash
# 推荐：通过环境变量传入
export MAAS_API_KEY="sk-your-key"

# 在代码中读取
import os
api_key = os.environ["MAAS_API_KEY"]
```

## 智能路由

MaaS 的路由引擎根据以下维度自动选择最优模型：

```
路由评分 = 
    成本分 × 权重₁ +
    延迟分 × 权重₂ +
    成功率分 × 权重₃

成本分 = 1 - (模型单价 / 同类最高单价)
延迟分 = 1 - (EWMA延迟 / 最大可接受延迟)
成功率分 = 近5分钟成功率
```

你可以在控制台「路由策略」中调整三个权重的比例，或直接在 API 请求中通过扩展 Header 指定优先级：

```bash
# 指定优先成本（会倾向选择便宜的模型）
curl ... -H "x-maas-priority: economy"

# 指定优先性能（会倾向选择效果最好的模型）
curl ... -H "x-maas-priority: performance"
```

## 语义缓存

语义缓存通过向量相似度匹配，复用语义相近请求的结果：

```
用户发送: "Python 如何读取文件？"
↓
缓存命中: 与 "怎么用Python打开一个文本文件" 的语义相似度 95%
↓
直接返回缓存结果，不消耗 Token！
```

**缓存默认开启**，可通过 Header 关闭（对实时性要求极高的场景）：

```bash
curl ... -H "x-maas-cache-enabled: false"
```

**缓存命中率参考：**
- 客服机器人（FAQ 场景）：40-60%
- 代码补全（高度定制化）：5-15%
- 文档处理（每次不同内容）：< 5%

---

# 第三部分：API 参考

## Chat Completions

### 请求

```
POST https://api.maas-platform.com/v1/chat/completions
```

**请求体：**

```json
{
  "model": "gpt-4o",
  "messages": [
    {"role": "system", "content": "You are a helpful assistant."},
    {"role": "user",   "content": "What is 2+2?"}
  ],
  "max_tokens": 1024,
  "temperature": 1.0,
  "top_p": 1.0,
  "stream": false,
  "stop": null,
  "user": "user_id_for_audit"
}
```

**MaaS 扩展字段：**

```json
{
  "model": "auto",
  "x-maas-fallback-models": "claude-3-5-haiku,qwen-plus",
  "x-maas-cache-enabled": true,
  "x-maas-priority": "economy"
}
```

### 响应

**非流式响应：**
```json
{
  "id": "chatcmpl-abc123",
  "object": "chat.completion",
  "created": 1747193600,
  "model": "gpt-4o",
  "choices": [{
    "index": 0,
    "message": {
      "role": "assistant",
      "content": "4"
    },
    "finish_reason": "stop"
  }],
  "usage": {
    "prompt_tokens": 26,
    "completion_tokens": 1,
    "total_tokens": 27
  },
  "x-maas-cache-hit": false,
  "x-maas-routing-model": "gpt-4o",
  "x-maas-request-id": "req_xyz789"
}
```

### 流式响应（stream: true）

每行为独立 SSE 事件：

```
data: {"id":"chatcmpl-abc123","choices":[{"delta":{"role":"assistant"},"index":0}]}

data: {"id":"chatcmpl-abc123","choices":[{"delta":{"content":"4"},"index":0}]}

data: {"id":"chatcmpl-abc123","choices":[{"delta":{},"finish_reason":"stop","index":0}]}

data: [DONE]
```

## Embeddings

```
POST https://api.maas-platform.com/v1/embeddings
```

```json
{
  "model": "text-embedding-3-small",
  "input": "The quick brown fox"
}
```

响应：
```json
{
  "object": "list",
  "data": [{
    "object": "embedding",
    "index": 0,
    "embedding": [0.0023, -0.0094, ...]
  }],
  "model": "text-embedding-3-small",
  "usage": {"prompt_tokens": 5, "total_tokens": 5}
}
```

## 可用模型列表

```
GET https://api.maas-platform.com/v1/models
```

响应中 `data` 字段为模型列表，重要字段：

| 字段 | 说明 |
|------|------|
| `id` | 模型名，用于 `model` 参数 |
| `context_length` | 最大上下文窗口（tokens） |
| `pricing.input` | 输入 Token 单价（元/1000 tokens） |
| `pricing.output` | 输出 Token 单价（元/1000 tokens） |
| `status` | `available` / `degraded` / `unavailable` |

## 错误码大全

| HTTP 状态 | 错误码 | 含义 | 处理方式 |
|----------|--------|------|---------|
| 400 | `invalid_request_error` | 请求参数不合法 | 检查请求体格式 |
| 401 | `authentication_error` | API Key 无效/已吊销 | 检查 API Key |
| 403 | `permission_error` | 无权限使用该模型或功能 | 检查 Key 权限配置 |
| 404 | `model_not_found` | 模型不存在 | 检查模型名称拼写 |
| 429 | `rate_limit_error` | 超过速率限制 | 降低请求频率，参考响应 Header |
| 429 | `quota_exceeded` | Token 配额耗尽 | 充值或申请提升配额 |
| 500 | `internal_error` | 平台内部错误 | 重试，持续则联系支持 |
| 503 | `service_unavailable` | 模型暂时不可用 | 重试或切换备用模型 |
| 504 | `timeout` | 请求超时 | 减小 max_tokens 后重试 |

---

# 第四部分：指南

## 流式输出最佳实践

```python
# Python 流式示例（with 上下文管理器，自动清理）
with client.chat.completions.create(
    model="gpt-4o",
    messages=[{"role": "user", "content": "写一篇500字文章"}],
    stream=True
) as stream:
    full_content = ""
    for chunk in stream:
        delta = chunk.choices[0].delta.content or ""
        full_content += delta
        print(delta, end="", flush=True)

print()  # 换行
print(f"Total chars: {len(full_content)}")
```

**注意事项：**
- 流式模式下，`usage` 信息在最后一条消息中（部分模型不返回，需自行计算）
- 网络断开后，SSE 不会自动重连，需要在应用层实现断线重连逻辑
- 超时设置建议比普通请求更长（建议 60-120s）

## 函数调用（Function Calling / Tool Use）

```python
tools = [{
    "type": "function",
    "function": {
        "name": "get_weather",
        "description": "获取指定城市的天气",
        "parameters": {
            "type": "object",
            "properties": {
                "city": {"type": "string", "description": "城市名称"}
            },
            "required": ["city"]
        }
    }
}]

response = client.chat.completions.create(
    model="gpt-4o",
    messages=[{"role": "user", "content": "北京今天天气怎么样？"}],
    tools=tools,
    tool_choice="auto"
)

# 检查是否触发了工具调用
if response.choices[0].finish_reason == "tool_calls":
    tool_call = response.choices[0].message.tool_calls[0]
    print(tool_call.function.name)       # get_weather
    print(tool_call.function.arguments)  # {"city": "北京"}
```

> **模型支持说明：** Function Calling 支持 GPT-4o、Claude-3.5、Qwen-Plus 及以上版本。Qwen-Turbo 不支持。

## 成本优化指南

### 选择合适的模型

```python
# 场景判断逻辑（伪代码）
def select_model(task_type, complexity):
    if task_type == "simple_qa":
        return "qwen-turbo"        # 最便宜，中文优化
    elif task_type == "code":
        return "deepseek-coder-v2" # 代码专用，性价比高
    elif complexity == "high":
        return "gpt-4o"            # 复杂任务用旗舰
    else:
        return "qwen-plus"         # 通用中等复杂度
```

### 控制 Token 消耗

```python
# 对话历史截断（只保留最近 N 轮）
def truncate_history(messages, max_rounds=5):
    system = [m for m in messages if m["role"] == "system"]
    dialog = [m for m in messages if m["role"] != "system"]
    # 保留最近 max_rounds × 2 条（用户+助手各一条为一轮）
    return system + dialog[-(max_rounds * 2):]
```

---

# 第五部分：变更日志

## V1.1.0（2026-07-01，计划）

- [ ] 支持 Batch API（批量处理，价格更低）
- [ ] 新增 Whisper 语音转文字接口
- [ ] 支持 GPT-4o 图片输入

## V1.0.0（2026-05-14）

- ✅ Chat Completions API（OpenAI 兼容）
- ✅ Embeddings API
- ✅ 智能路由（成本/延迟/成功率多维评分）
- ✅ 语义缓存（Milvus 向量相似度匹配）
- ✅ 精调 API（LoRA/QLoRA/SFT）
- ✅ 实时用量查询 API
- ✅ Python / Go / JS / Java SDK 示例
