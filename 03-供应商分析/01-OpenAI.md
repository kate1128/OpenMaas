# OpenAI 供应商分析

**文档版本：** V1.0  
**最后更新：** 2026-05-20  
**官网：** https://platform.openai.com  
**定价页：** https://openai.com/pricing

---

## 1. 基本信息

| 项目 | 内容 |
|------|------|
| 公司 | OpenAI |
| 总部 | 美国旧金山 |
| 国内可访问 | ❌ 需代理或香港节点 |
| API 格式 | **行业标准**（其他厂商均以此为参考实现） |
| 计费单位 | per 1M tokens |
| 结算货币 | USD |
| 支付方式 | 信用卡充值（预付），企业可申请授信 |

---

## 2. API 接入

### 2.1 认证

```http
Authorization: Bearer sk-proj-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

API Key 前缀：
- `sk-proj-...`：项目级 Key（推荐，可限制模型/IP）  
- `sk-org-...`：组织级 Key（旧版，不推荐新建）

### 2.2 Base URL

```
https://api.openai.com/v1
```

**Chat Completions：**
```
POST https://api.openai.com/v1/chat/completions
```

**Embeddings：**
```
POST https://api.openai.com/v1/embeddings
```

**Models 列表：**
```
GET https://api.openai.com/v1/models
```

### 2.3 SDK

```bash
pip install openai          # Python
npm install openai          # Node.js
```

```python
from openai import OpenAI
client = OpenAI(api_key="sk-proj-xxx")
```

---

## 3. 调用格式

### 3.1 Chat Completions 请求

```json
{
  "model": "gpt-4o",
  "messages": [
    {"role": "system", "content": "You are a helpful assistant."},
    {"role": "user",   "content": "Hello!"}
  ],
  "temperature": 0.7,
  "max_tokens": 2048,
  "stream": false,
  "top_p": 1.0,
  "frequency_penalty": 0,
  "presence_penalty": 0,
  "response_format": {"type": "json_object"}  // 可选，强制 JSON 输出
}
```

### 3.2 Chat Completions 响应

```json
{
  "id": "chatcmpl-abc123",
  "object": "chat.completion",
  "created": 1716200000,
  "model": "gpt-4o-2024-11-20",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Hello! How can I help you today?"
      },
      "finish_reason": "stop",
      "logprobs": null
    }
  ],
  "usage": {
    "prompt_tokens": 18,
    "completion_tokens": 10,
    "total_tokens": 28,
    "prompt_tokens_details": {
      "cached_tokens": 0,
      "audio_tokens": 0
    }
  },
  "system_fingerprint": "fp_abc123"
}
```

### 3.3 流式输出（stream: true）

```
data: {"id":"chatcmpl-abc","object":"chat.completion.chunk","choices":[{"delta":{"role":"assistant","content":""},"index":0}]}
data: {"id":"chatcmpl-abc","object":"chat.completion.chunk","choices":[{"delta":{"content":"Hello"},"index":0}]}
data: {"id":"chatcmpl-abc","object":"chat.completion.chunk","choices":[{"delta":{},"finish_reason":"stop","index":0}],"usage":{"prompt_tokens":18,"completion_tokens":5,"total_tokens":23}}
data: [DONE]
```

### 3.4 Function Calling / Tools

```json
{
  "model": "gpt-4o",
  "messages": [{"role": "user", "content": "北京天气怎么样？"}],
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "get_weather",
        "description": "获取指定城市的实时天气",
        "parameters": {
          "type": "object",
          "properties": {
            "city": {"type": "string", "description": "城市名"}
          },
          "required": ["city"]
        }
      }
    }
  ],
  "tool_choice": "auto"
}
```

### 3.5 Vision（多模态）

```json
{
  "model": "gpt-4o",
  "messages": [
    {
      "role": "user",
      "content": [
        {"type": "text", "text": "描述这张图片"},
        {
          "type": "image_url",
          "image_url": {
            "url": "https://example.com/image.jpg",
            "detail": "high"
          }
        }
      ]
    }
  ]
}
```

### 3.6 Embeddings

```json
// 请求
{
  "model": "text-embedding-3-small",
  "input": ["Hello world", "How are you?"],
  "encoding_format": "float"
}

// 响应
{
  "object": "list",
  "data": [
    {"object": "embedding", "index": 0, "embedding": [0.0023, -0.0067, ...]}
  ],
  "usage": {"prompt_tokens": 6, "total_tokens": 6}
}
```

---

## 4. 模型列表

### 4.1 GPT-4o 系列（旗舰多模态）

| 模型 ID | 上下文 | 输入 ¥/M | 输出 ¥/M | 特点 |
|---------|-------|--------:|--------:|------|
| `gpt-4o` | 128k | ≈36 | ≈109 | 旗舰，视觉+函数调用 |
| `gpt-4o-2024-11-20` | 128k | ≈36 | ≈109 | 指定版本（推荐生产用） |
| `gpt-4o-mini` | 128k | ≈1.1 | ≈4.4 | 轻量快速，性价比高 |
| `gpt-4o-mini-2024-07-18` | 128k | ≈1.1 | ≈4.4 | 指定版本 |

### 4.2 o 系列（推理模型）

| 模型 ID | 上下文 | 输入 ¥/M | 输出 ¥/M | 特点 |
|---------|-------|--------:|--------:|------|
| `o3` | 200k | ≈145 | ≈580 | 最强推理，最贵 |
| `o3-mini` | 200k | ≈8 | ≈32 | 推理小模型，性价比 |
| `o1` | 200k | ≈109 | ≈435 | 上一代旗舰推理 |
| `o1-mini` | 128k | ≈22 | ≈88 | 上一代推理小模型 |
| `o4-mini` | 200k | ≈8 | ≈32 | 最新小推理模型 |

> 推理模型会产生内部 `reasoning_tokens`（thinking），计入 completion_tokens 计费但不输出文本。

### 4.3 Embedding 模型

| 模型 ID | 维度 | 价格 ¥/M | 特点 |
|---------|------|--------:|------|
| `text-embedding-3-small` | 1536 | ≈0.15 | 轻量，适合大批量 |
| `text-embedding-3-large` | 3072 | ≈0.94 | 高精度 |
| `text-embedding-ada-002` | 1536 | ≈0.72 | 旧版，保持兼容 |

### 4.4 Batch API（批量异步，半价）

适合非实时任务（数据处理、Embedding 批量生成）：
- 提交 `.jsonl` 文件 → 24 小时内返回结果
- **价格为同步 API 的 50%**
- 接口：`POST /v1/batches`

---

## 5. 价格分析

### 5.1 成本估算示例

| 场景 | 模型 | 每次调用 Tokens | 每次费用 | 10万次/月 |
|------|------|--------------|-------:|----------:|
| 客服问答 | gpt-4o-mini | 500 in + 200 out | ≈¥0.0014 | ≈¥140 |
| 代码生成 | gpt-4o | 1k in + 500 out | ≈¥0.09 | ≈¥9,000 |
| 文档摘要 | gpt-4o | 3k in + 500 out | ≈¥0.16 | ≈¥16,000 |
| Embedding | text-embedding-3-small | 500 tokens | ≈¥0.0001 | ≈¥10 |

### 5.2 缓存机制（Prompt Cache）

- 超过 1024 tokens 的 prompt 前缀自动缓存（5-10分钟有效期）
- **缓存命中：输入价格减半**
- 适用于固定 System Prompt 的场景

### 5.3 与国产模型价格对比

| 场景 | OpenAI gpt-4o | DeepSeek-V3 | 节省比例 |
|------|:------------:|:-----------:|:-------:|
| 旗舰模型旗舰任务 | ¥0.09/次 | ¥0.003/次 | ~97% |
| 快速任务（mini） | ¥0.0014/次 | ¥0.0015/次 | 相当 |

---

## 6. 特殊能力

| 能力 | 支持情况 |
|------|---------|
| 多模态（图片理解） | ✅ gpt-4o / o1 / o3 |
| 视频理解 | ⚠️ 有限支持（通过 base64 帧） |
| 函数调用 | ✅ 所有 chat 模型 |
| 结构化输出（JSON Schema） | ✅ gpt-4o / gpt-4o-mini |
| 流式输出 | ✅ 所有 chat 模型 |
| Batch API | ✅ 半价异步处理 |
| 文件上传（Assistants） | ✅ Files API |
| 语音转文字 | ✅ Whisper |
| 文字转语音 | ✅ TTS API |
| 图片生成 | ✅ DALL·E 3 |
| Prompt Caching | ✅ 自动，输入减半价 |
| 精调（Fine-tuning） | ✅ gpt-4o-mini / gpt-3.5-turbo |

---

## 7. 速率限制（Rate Limits）

速率限制按 **Tier** 分级，注册账号默认为 Tier 1，充值后自动升级：

| Tier | 条件 | RPM | TPM |
|------|------|----:|----:|
| Free | 新注册 | 3 | 40,000 |
| Tier 1 | 充值 $5 | 500 | 200,000 |
| Tier 2 | 充值 $50 | 5,000 | 2,000,000 |
| Tier 3 | 充值 $100 | 5,000 | 4,000,000 |
| Tier 4 | 充值 $250 | 10,000 | 10,000,000 |
| Tier 5 | 充值 $1,000 | 10,000 | 150,000,000 |

> RPM = Requests Per Minute；TPM = Tokens Per Minute

---

## 8. MaaS 接入评估

### 8.1 接入方案

```python
# 最简接入：换 base_url 即可
from openai import OpenAI

client = OpenAI(
    api_key="sk-proj-xxx",  # 替换为 OpenAI Key
    base_url="https://api.openai.com/v1"  # 默认值，可省略
)
```

**MaaS 侧配置（vendor config）：**
```yaml
vendor: openai
base_url: "https://api.openai.com/v1"
auth_type: bearer_token
api_key_env: OPENAI_API_KEY
timeout: 60s
retry:
  max_attempts: 3
  backoff: exponential
```

### 8.2 Adapter 复杂度：极低

- 请求/响应格式即 MaaS 标准格式，**无需转换**
- 流式格式即标准 SSE + `[DONE]`
- 唯一需要处理的：`reasoning_tokens`（o 系列）字段在 usage 中额外出现

### 8.3 注意事项

1. **网络访问**：国内直连不通，需要配置香港/海外代理节点或通过中转服务
2. **Key 安全**：建议使用 Project Key（`sk-proj-`），并设置 IP 白名单和模型限制
3. **Batch API 利用**：对于离线任务，优先使用 Batch API 节省 50% 成本
4. **模型版本锁定**：生产环境建议指定 dated 版本（如 `gpt-4o-2024-11-20`），避免模型升级导致行为变化
5. **Token 计算**：使用 `tiktoken` 库在客户端预估 token 数，避免超出 context window 报错
6. **o 系列注意**：`o1/o3` 不支持 `system` role，需将 system 内容合并到第一条 user 消息；`temperature` 参数无效

---

## 9. 常见错误码

| HTTP 状态码 | 错误类型 | 处理建议 |
|:----------:|---------|---------|
| 400 | invalid_request_error | 检查请求参数 |
| 401 | authentication_error | 检查 API Key |
| 403 | permission_error | Key 无权限访问该模型 |
| 429 | rate_limit_error | 退避重试，检查 Tier 限制 |
| 500 | server_error | 重试，持续则联系 OpenAI |
| 503 | service_unavailable | 退避重试 |
