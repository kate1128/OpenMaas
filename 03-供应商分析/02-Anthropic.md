# Anthropic (Claude) 供应商分析

**文档版本：** V1.0  
**最后更新：** 2026-05-20  
**官网：** https://www.anthropic.com  
**API 文档：** https://docs.anthropic.com  
**定价页：** https://www.anthropic.com/pricing

---

## 1. 基本信息

| 项目 | 内容 |
|------|------|
| 公司 | Anthropic |
| 总部 | 美国旧金山 |
| 国内可访问 | ❌ 需代理 |
| API 格式 | **自有格式**（与 OpenAI 存在差异） |
| OpenAI 兼容 | ⚠️ 有兼容层，但 system/content 结构不同 |
| 计费单位 | per 1M tokens |
| 结算货币 | USD |
| 支付方式 | 信用卡充值（预付） |

---

## 2. API 接入

### 2.1 认证

```http
x-api-key: sk-ant-api03-xxxxxxxxxxxxxxxx
anthropic-version: 2023-06-01
content-type: application/json
```

> **注意：** Anthropic 使用 `x-api-key` Header，而非 `Authorization: Bearer`

### 2.2 Base URL

```
https://api.anthropic.com/v1
```

**Chat（Messages API）：**
```
POST https://api.anthropic.com/v1/messages
```

### 2.3 SDK

```bash
pip install anthropic        # Python
npm install @anthropic-ai/sdk # Node.js
```

```python
import anthropic
client = anthropic.Anthropic(api_key="sk-ant-api03-xxx")
```

---

## 3. 调用格式

> **关键差异：** Anthropic 的格式与 OpenAI 存在以下核心差异：  
> 1. `system` 是顶层字段，不在 `messages` 数组中  
> 2. 响应的 `content` 是数组而非字符串  
> 3. 需要传 `max_tokens`（OpenAI 中可选）  
> 4. 请求 Header 不同

### 3.1 Messages API 请求（Anthropic 原生格式）

```json
{
  "model": "claude-3-5-sonnet-20241022",
  "max_tokens": 2048,
  "system": "You are a helpful assistant.",
  "messages": [
    {"role": "user",      "content": "Hello!"},
    {"role": "assistant", "content": "Hello! How can I help you?"},
    {"role": "user",      "content": "Tell me about AI."}
  ],
  "temperature": 0.7,
  "top_p": 0.9,
  "stream": false
}
```

### 3.2 Messages API 响应（原生格式）

```json
{
  "id": "msg_abc123",
  "type": "message",
  "role": "assistant",
  "model": "claude-3-5-sonnet-20241022",
  "content": [
    {
      "type": "text",
      "text": "AI (Artificial Intelligence) refers to..."
    }
  ],
  "stop_reason": "end_turn",
  "stop_sequence": null,
  "usage": {
    "input_tokens": 25,
    "output_tokens": 150,
    "cache_creation_input_tokens": 0,
    "cache_read_input_tokens": 0
  }
}
```

### 3.3 流式输出

```
event: message_start
data: {"type":"message_start","message":{"id":"msg_abc","type":"message","role":"assistant","model":"claude-3-5-sonnet-20241022","content":[]}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":5}}

event: message_stop
data: {"type":"message_stop"}
```

> **注意：** 流式格式与 OpenAI 的 SSE 格式差异较大，需要单独处理。

### 3.4 Function Calling（Tools）

```json
{
  "model": "claude-3-5-sonnet-20241022",
  "max_tokens": 1024,
  "tools": [
    {
      "name": "get_weather",
      "description": "Get weather for a city",
      "input_schema": {
        "type": "object",
        "properties": {
          "city": {"type": "string", "description": "City name"}
        },
        "required": ["city"]
      }
    }
  ],
  "messages": [{"role": "user", "content": "北京天气？"}]
}
```

工具调用响应中 content 会包含 `tool_use` 类型 block：
```json
{
  "content": [
    {
      "type": "tool_use",
      "id": "toolu_abc",
      "name": "get_weather",
      "input": {"city": "北京"}
    }
  ]
}
```

### 3.5 Vision（多模态）

```json
{
  "model": "claude-3-5-sonnet-20241022",
  "max_tokens": 1024,
  "messages": [
    {
      "role": "user",
      "content": [
        {
          "type": "image",
          "source": {
            "type": "base64",
            "media_type": "image/jpeg",
            "data": "/9j/4AAQSkZJRgAB..."
          }
        },
        {"type": "text", "text": "描述这张图片"}
      ]
    }
  ]
}
```

### 3.6 Prompt Caching（缓存优化）

```json
{
  "system": [
    {
      "type": "text",
      "text": "你是一个专业的法律顾问，以下是完整的合同文本：\n\n...(10000 tokens的固定内容)...",
      "cache_control": {"type": "ephemeral"}  // 标记此处做缓存断点
    }
  ],
  "messages": [{"role": "user", "content": "第三条款是什么意思？"}]
}
```

缓存命中时 `cache_read_input_tokens` 减价 **90%**，缓存写入额外收费 25%（一次）。

---

## 4. 模型列表

### 4.1 Claude 3.5 系列（推荐使用）

| 模型 ID | 上下文 | 输入 ¥/M | 输出 ¥/M | 特点 |
|---------|-------|--------:|--------:|------|
| `claude-3-5-sonnet-20241022` | 200k | ≈22 | ≈109 | 旗舰，综合最强 |
| `claude-3-5-haiku-20241022` | 200k | ≈5.8 | ≈29 | 快速低价，适合高频 |

### 4.2 Claude 3 系列

| 模型 ID | 上下文 | 输入 ¥/M | 输出 ¥/M | 特点 |
|---------|-------|--------:|--------:|------|
| `claude-3-opus-20240229` | 200k | ≈109 | ≈543 | 最强但最贵，已被 3.5 超越 |
| `claude-3-sonnet-20240229` | 200k | ≈22 | ≈109 | 旧版均衡 |
| `claude-3-haiku-20240307` | 200k | ≈1.8 | ≈8.7 | 旧版快速 |

### 4.3 Claude 4 系列（最新）

| 模型 ID | 上下文 | 输入 ¥/M | 输出 ¥/M | 特点 |
|---------|-------|--------:|--------:|------|
| `claude-sonnet-4-5` | 200k | ≈22 | ≈109 | 最新旗舰 |
| `claude-haiku-4-5` | 200k | ≈5.8 | ≈29 | 最新快速版 |

---

## 5. 价格分析

### 5.1 Prompt Caching 节省估算

适用场景：长文档分析、固定 System Prompt + 变化用户问题

| 场景 | 未缓存 | 缓存命中 | 节省比例 |
|------|:------:|:-------:|:-------:|
| 5k System + 100 User | ≈¥0.115 | ≈¥0.018 | ~84% |
| 10k Document + 50 Query | ≈¥0.22 | ≈¥0.024 | ~89% |

### 5.2 与 OpenAI 价格对比

| 场景 | claude-3-5-sonnet | gpt-4o | 差异 |
|------|:----------------:|:------:|:---:|
| 旗舰对话（1k in + 500 out） | ≈¥0.076 | ≈¥0.09 | Claude 便宜 ~15% |
| 快速版（1k in + 500 out） | haiku ≈¥0.021 | mini ≈¥0.003 | GPT 便宜 ~85% |

### 5.3 Batch API（异步半价，Beta）

与 OpenAI 类似，支持提交批量任务，异步处理，价格约为同步的 **50%**。

---

## 6. 特殊能力

| 能力 | 支持情况 |
|------|---------|
| 多模态（图片理解） | ✅ 所有 Claude 3/3.5 |
| 函数调用（Tools） | ✅ 所有模型 |
| 流式输出 | ✅ 所有模型 |
| Prompt Caching | ✅ 显著降低长 prompt 成本 |
| 结构化输出 | ⚠️ 无原生 JSON schema 强制，需 prompt 引导 |
| 精调（Fine-tuning） | ❌ 目前不支持 |
| Embedding | ❌ 无 Embedding API |
| 超长上下文 | ✅ 200k tokens |
| Computer Use | ✅ claude-3-5-sonnet（Beta，操控电脑） |

---

## 7. 速率限制

| Tier | 条件 | RPM | TPM/天 |
|------|------|----:|------:|
| Free | 注册即有 | 5 | 25k |
| Tier 1 | 充值 $5 | 50 | 无限制 |
| Tier 2 | 充值 $40，7天后 | 1,000 | 无限制 |
| Tier 3 | 消费 $250，30天后 | 2,000 | 无限制 |
| Tier 4 | 消费 $500，30天后 | 4,000 | 无限制 |

---

## 8. MaaS 接入评估

### 8.1 Adapter 核心差异处理

**差异1：system role 转换**
```python
# OpenAI 格式（输入）
messages = [
    {"role": "system", "content": "你是助手"},
    {"role": "user", "content": "你好"}
]

# Anthropic 格式（需转换）
system = "你是助手"
messages = [{"role": "user", "content": "你好"}]
```

**差异2：响应格式标准化**
```python
# Anthropic 响应 → OpenAI 格式
def normalize_response(anthropic_resp):
    return {
        "id": anthropic_resp["id"],
        "object": "chat.completion",
        "choices": [{
            "index": 0,
            "message": {
                "role": "assistant",
                "content": anthropic_resp["content"][0]["text"]  # 取第一个 text block
            },
            "finish_reason": "stop" if anthropic_resp["stop_reason"] == "end_turn" else anthropic_resp["stop_reason"]
        }],
        "usage": {
            "prompt_tokens": anthropic_resp["usage"]["input_tokens"],
            "completion_tokens": anthropic_resp["usage"]["output_tokens"],
            "total_tokens": anthropic_resp["usage"]["input_tokens"] + anthropic_resp["usage"]["output_tokens"]
        }
    }
```

**差异3：Header 格式**
```python
headers = {
    "x-api-key": api_key,            # 不同于 Authorization: Bearer
    "anthropic-version": "2023-06-01",
    "content-type": "application/json"
}
```

### 8.2 接入配置

```yaml
vendor: anthropic
base_url: "https://api.anthropic.com/v1"
auth_type: api_key_header          # Header 名 x-api-key
api_key_env: ANTHROPIC_API_KEY
extra_headers:
  anthropic-version: "2023-06-01"
adapter: anthropic_v1              # 需要专用 Adapter
timeout: 120s                      # 长文本生成需要更长超时
```

### 8.3 注意事项

1. **网络访问**：国内需代理，建议香港节点
2. **max_tokens 必填**：不指定会报错（OpenAI 中可选）
3. **system role 单独处理**：不能放在 messages 数组里
4. **流式事件格式**：完全不同于 OpenAI，需实现专用 SSE 解析器
5. **content 数组**：响应的 content 是数组（支持多 block），提取文本需取 `type: text` 的 block
6. **Tool Use 轮次**：函数调用后需将 `tool_result` 放回 messages 继续对话，格式与 OpenAI 不同
7. **没有 Embedding**：如需向量检索，需另选厂商

---

## 9. 常见错误码

| 状态码 | 错误类型 | 处理建议 |
|:------:|---------|---------|
| 400 | invalid_request_error | 检查 max_tokens / messages 格式 |
| 401 | authentication_error | 检查 x-api-key |
| 403 | permission_error | 账号未开通此模型 |
| 429 | rate_limit_error | 退避重试，检查 Tier |
| 529 | overloaded_error | 服务过载，退避重试 |
| 500 | api_error | 重试 |
