# Google Gemini 供应商分析

**文档版本：** V1.0  
**最后更新：** 2026-05-20  
**官网：** https://ai.google.dev  
**API 文档：** https://ai.google.dev/api  
**定价页：** https://ai.google.dev/pricing

---

## 1. 基本信息

| 项目 | 内容 |
|------|------|
| 公司 | Google / Google DeepMind |
| API 平台 | Google AI Studio（Gemini API） / Vertex AI |
| 国内可访问 | ❌ 需代理 |
| API 格式 | 自有格式，但提供 **OpenAI 兼容端点** |
| 核心优势 | **超长上下文**（1M-2M tokens）、多模态原生支持、价格低 |
| 计费单位 | per 1M tokens / 图片按张 / 音频按分钟 |
| 结算货币 | USD |
| 支付方式 | Google 账户充值 / GCP 账单 |

---

## 2. API 接入

### 2.1 两种 API 接入方式

**方式一：Google AI Studio API（推荐测试和中小流量）**
- 简单 API Key 鉴权
- 接入点：`https://generativelanguage.googleapis.com/v1beta`
- 有免费 tier（每天限量请求）

**方式二：Vertex AI API（推荐企业/高流量）**
- GCP OAuth 鉴权，与 GCP 计费整合
- 更高配额，支持 VPC 私网访问
- 接入点：`https://{region}-aiplatform.googleapis.com/v1`

本文主要介绍 **Google AI Studio API（方式一）**，适合 MaaS 接入。

### 2.2 认证（API Key 方式）

```http
# 方式一：Query 参数
GET https://generativelanguage.googleapis.com/v1beta/models?key=AIzaSy-xxx

# 方式二：Header（OpenAI 兼容端点用）
Authorization: Bearer AIzaSy-xxx
```

### 2.3 Base URL

**原生格式：**
```
https://generativelanguage.googleapis.com/v1beta
```

**OpenAI 兼容端点（推荐用于 MaaS）：**
```
https://generativelanguage.googleapis.com/v1beta/openai
```

### 2.4 SDK

```bash
pip install google-generativeai    # Google 官方 SDK
pip install openai                  # 使用 OpenAI SDK + 兼容端点
```

```python
# 方式一：OpenAI SDK（推荐，用于 MaaS 接入）
from openai import OpenAI

client = OpenAI(
    api_key="AIzaSy-xxx",
    base_url="https://generativelanguage.googleapis.com/v1beta/openai"
)

response = client.chat.completions.create(
    model="gemini-2.0-flash",
    messages=[{"role": "user", "content": "你好"}]
)
```

---

## 3. 调用格式

### 3.1 通过 OpenAI 兼容端点（推荐）

```json
{
  "model": "gemini-2.0-flash",
  "messages": [
    {"role": "system", "content": "你是一个有帮助的助手"},
    {"role": "user",   "content": "介绍一下 Gemini"}
  ],
  "temperature": 0.7,
  "max_tokens": 4096,
  "stream": false
}
```

> OpenAI 兼容端点已支持大部分 OpenAI 参数，但以下有限制：  
> - `logprobs` 不支持
> - `function_calling_config` 行为略有不同

### 3.2 原生 generateContent API

```json
POST https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent?key=AIzaSy-xxx

{
  "contents": [
    {
      "role": "user",
      "parts": [{"text": "介绍一下 Gemini"}]
    }
  ],
  "systemInstruction": {
    "parts": [{"text": "你是一个有帮助的助手"}]
  },
  "generationConfig": {
    "temperature": 0.7,
    "maxOutputTokens": 4096,
    "topP": 0.95
  }
}
```

### 3.3 原生格式响应

```json
{
  "candidates": [
    {
      "content": {
        "parts": [{"text": "Gemini 是 Google 开发的..."}],
        "role": "model"
      },
      "finishReason": "STOP",
      "index": 0
    }
  ],
  "usageMetadata": {
    "promptTokenCount": 20,
    "candidatesTokenCount": 150,
    "totalTokenCount": 170
  }
}
```

### 3.4 Vision（多模态）

```json
{
  "contents": [
    {
      "role": "user",
      "parts": [
        {
          "inlineData": {
            "mimeType": "image/jpeg",
            "data": "/9j/4AAQ..."  // base64
          }
        },
        {"text": "描述这张图片"}
      ]
    }
  ]
}
```

OpenAI 兼容端点格式：
```json
{
  "model": "gemini-2.0-flash",
  "messages": [
    {
      "role": "user",
      "content": [
        {"type": "image_url", "image_url": {"url": "data:image/jpeg;base64,/9j/4AAQ..."}},
        {"type": "text", "text": "描述这张图片"}
      ]
    }
  ]
}
```

### 3.5 Embedding（原生格式）

```json
POST https://generativelanguage.googleapis.com/v1beta/models/text-embedding-004:embedContent?key=xxx

{
  "model": "models/text-embedding-004",
  "content": {"parts": [{"text": "Hello world"}]}
}
```

> **注意：** Embedding 目前在 OpenAI 兼容端点中有限支持。

---

## 4. 模型列表

### 4.1 Gemini 2.0 系列（最新）

| 模型 ID | 上下文 | 输入 ¥/M | 输出 ¥/M | 特点 |
|---------|-------|--------:|--------:|------|
| `gemini-2.0-flash` | 1M | ≈0.55 | ≈2.2 | **主力**，快速低价，1M上下文 |
| `gemini-2.0-flash-lite` | 1M | ≈0.26 | ≈1.1 | 极低价 |
| `gemini-2.0-flash-thinking` | 32k | ≈2.5 | ≈10.9 | 推理版本 |
| `gemini-2.5-pro` | 1M | ≈18 | ≈72（>200k另计） | 最强版本 |

### 4.2 Gemini 1.5 系列

| 模型 ID | 上下文 | 输入 ¥/M | 输出 ¥/M | 特点 |
|---------|-------|--------:|--------:|------|
| `gemini-1.5-pro` | 2M | ≈9（≤128k）/ ≈18（>128k） | ≈36 / ≈72 | **2M超长上下文** |
| `gemini-1.5-flash` | 1M | ≈0.54 | ≈2.2 | 快速低价 |
| `gemini-1.5-flash-8b` | 1M | ≈0.27 | ≈1.1 | 极速 |

### 4.3 免费 Tier（Google AI Studio）

| 模型 | 免费 RPM | 免费 TPD（每天） |
|------|:-------:|:--------------:|
| gemini-2.0-flash | 15 | 1,500,000 |
| gemini-1.5-flash | 15 | 1,500,000 |
| gemini-1.5-pro | 2 | 50,000 |

### 4.4 Embedding 模型

| 模型 ID | 维度 | 价格 ¥/M | 特点 |
|---------|------|--------:|------|
| `text-embedding-004` | 768 | ≈0 (免费) | 当前主力 |
| `text-multilingual-embedding-002` | 768 | ≈0 | 多语言 |

---

## 5. 价格分析

### 5.1 超长上下文成本分析

| 任务 | gemini-1.5-pro（2M） | claude-3-5-sonnet（200k） | moonshot-128k |
|------|:-------------------:|:------------------------:|:-------------:|
| 100k tokens 文档 | ≈¥1.3 | ≈¥2.9 | ≈¥7.3 |
| 500k tokens 文档 | ≈¥9 | 不支持 | 不支持 |
| 1M tokens 文档 | ≈¥18 | 不支持 | 不支持 |

**Gemini 1.5 Pro 在超长文档场景有明显价格优势。**

### 5.2 Flash 快速版性价比

| 对比 | gemini-2.0-flash | gpt-4o-mini | deepseek-chat |
|------|:----------------:|:-----------:|:-------------:|
| 输入 ¥/M | **≈0.55** | ≈1.1 | 1 |
| 输出 ¥/M | **≈2.2** | ≈4.4 | 2 |
| 上下文 | **1M** | 128k | 64k |

---

## 6. 特殊能力

| 能力 | 支持情况 |
|------|---------|
| 多模态（图片/视频/音频） | ✅ 原生多模态，最全面 |
| 视频理解 | ✅ 直接传视频文件 |
| 音频理解 | ✅ 直接传音频文件 |
| 函数调用 | ✅ 所有模型 |
| 流式输出 | ✅ 所有模型 |
| Embedding | ✅ text-embedding-004 |
| 超长上下文 | ✅ 最高 2M tokens |
| 代码执行（沙箱）| ✅ Code Execution 工具 |
| Google Search 工具 | ✅ Grounding with Google Search |
| 推理模型 | ✅ Flash Thinking |
| 免费 Tier | ✅ 有每日免费限额 |
| 精调 | ✅ Vertex AI 支持 |

---

## 7. MaaS 接入评估

### 7.1 推荐使用 OpenAI 兼容端点

```yaml
vendor: google_gemini
base_url: "https://generativelanguage.googleapis.com/v1beta/openai"
auth_type: bearer_token
api_key_env: GEMINI_API_KEY
adapter: openai_compatible         # 兼容端点基本兼容
timeout: 120s                       # 超长上下文需要更长超时
```

### 7.2 注意兼容性限制

OpenAI 兼容端点与原生差异（需 Adapter 处理）：

| 功能 | 兼容端点支持 | 处理方式 |
|------|:----------:|---------|
| system role | ✅ | 自动转换为 systemInstruction |
| vision | ✅ | 格式与 OpenAI 相同 |
| function calling | ✅ | 大部分兼容 |
| logprobs | ❌ | 忽略此参数 |
| seed | ⚠️ | 有限支持 |
| response_format | ⚠️ | json_object 支持 |

### 7.3 超长上下文路由建议

```yaml
# 超长文档自动路由到 Gemini
routing_rules:
  - condition: "input_tokens >= 200000"
    route_to: gemini-1.5-pro
    reason: "唯一支持 200k+ tokens 的模型"
  - condition: "input_tokens >= 50000 AND cost_priority == high"
    route_to: gemini-2.0-flash
    reason: "1M 上下文，价格低于其他厂商"
```

### 7.4 注意事项

1. **国内访问**：需要代理，建议新加坡/香港 GCP 节点
2. **安全过滤（Safety Filters）**：Gemini 有比较严格的内容过滤，可能拒绝一些合理请求，企业版可调整
3. **每次请求返回多个 candidates**：默认 1 个，可配置多个候选答案，MaaS 层取第一个即可
4. **视频/音频计费**：非 Token 计费，按分钟/秒计算，需单独处理计费逻辑
5. **Vertex AI vs AI Studio API**：高流量建议用 Vertex AI，配额更高，与 GCP VPC 集成

---

## 8. 常见错误码

| 错误类型 | 含义 | 处理建议 |
|---------|-----|---------|
| INVALID_ARGUMENT | 参数错误 | 检查请求格式 |
| PERMISSION_DENIED | API Key 无权限 | 检查 Key |
| RESOURCE_EXHAUSTED | 超出配额 | 升级计划或退避 |
| SAFETY | 内容被安全过滤 | 调整 prompt |
| 429 | 速率限制 | 退避重试 |
