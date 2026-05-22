# 月之暗面（Moonshot / Kimi）供应商分析

**文档版本：** V1.0  
**最后更新：** 2026-05-20  
**官网：** https://platform.moonshot.cn  
**API 文档：** https://platform.moonshot.cn/docs  
**定价页：** https://platform.moonshot.cn/docs/pricing

---

## 1. 基本信息

| 项目 | 内容 |
|------|------|
| 公司 | 北京月之暗面科技有限公司 |
| 产品 | Moonshot API / Kimi 大模型 |
| 国内可访问 | ✅ 直连可用 |
| API 格式 | **OpenAI 完全兼容** |
| 核心优势 | **超长上下文**（最大 128k），中文对话能力强 |
| 计费单位 | per 1M tokens |
| 结算货币 | CNY（人民币） |
| 支付方式 | 微信/支付宝充值（预付） |

---

## 2. API 接入

### 2.1 认证

```http
Authorization: Bearer sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
Content-Type: application/json
```

### 2.2 Base URL

```
https://api.moonshot.cn/v1
```

**Chat Completions：**
```
POST https://api.moonshot.cn/v1/chat/completions
```

### 2.3 SDK

直接使用 OpenAI SDK：

```python
from openai import OpenAI

client = OpenAI(
    api_key="sk-xxx",
    base_url="https://api.moonshot.cn/v1"
)

response = client.chat.completions.create(
    model="moonshot-v1-8k",
    messages=[
        {"role": "system", "content": "你是 Kimi，一个有帮助的 AI 助手"},
        {"role": "user", "content": "你好"}
    ]
)
```

---

## 3. 调用格式

### 3.1 标准请求（与 OpenAI 完全一致）

```json
{
  "model": "moonshot-v1-32k",
  "messages": [
    {"role": "system", "content": "你是一个有帮助的助手"},
    {"role": "user",   "content": "帮我总结这篇文章..."}
  ],
  "temperature": 0.3,
  "max_tokens": 4096,
  "stream": false,
  "top_p": 1.0
}
```

### 3.2 文件上传与长文本处理

Moonshot 支持上传文件（PDF/DOCX/TXT 等），将文件内容注入上下文：

```python
# 上传文件
with open("document.pdf", "rb") as f:
    file_obj = client.files.create(file=f, purpose="file-extract")

# 获取文件内容（转为文本）
file_content = client.files.content(file_obj.id)

# 在 messages 中引用
messages = [
    {
        "role": "system",
        "content": "你是文档分析助手",
    },
    {
        "role": "system",
        "content": file_content.text  # 注入文件文本
    },
    {
        "role": "user",
        "content": "请总结这份文档的主要内容"
    }
]
```

### 3.3 Function Calling

```json
{
  "model": "moonshot-v1-8k",
  "messages": [{"role": "user", "content": "查询北京天气"}],
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "get_weather",
        "description": "获取城市天气信息",
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

### 3.4 流式输出（标准 SSE）

与 OpenAI 格式完全一致：
```
data: {"id":"chat-xxx","choices":[{"delta":{"content":"你好"},"index":0}]}
...
data: [DONE]
```

---

## 4. 模型列表

### 4.1 Moonshot-v1 系列（文本对话）

| 模型 ID | 上下文 | 输入 ¥/M | 输出 ¥/M | 特点 |
|---------|-------|--------:|--------:|------|
| `moonshot-v1-8k` | 8k | 12 | 12 | 短对话，低价 |
| `moonshot-v1-32k` | 32k | 24 | 24 | 中等文档 |
| `moonshot-v1-128k` | 128k | 60 | 60 | **超长文档旗舰** |

> 定价对称（输入 = 输出），成本估算简单。

### 4.2 Moonshot-v1-AutoK（自适应 Context）

`moonshot-v1-auto` 系列会根据实际 prompt 长度自动选择对应版本计费，简化选型：

```python
# 用 auto 自动选择最短够用的上下文窗口
model="moonshot-v1-auto"  # 自动路由到 8k/32k/128k 版本
```

---

## 5. 价格分析

### 5.1 Moonshot 定价特点

- **输入输出同价**（不同于 OpenAI 输出更贵的模式）
- 按上下文长度分档，**长文档场景越经济**
- 没有 Prompt Caching 机制

### 5.2 长文档场景成本对比

| 场景 | 模型 | 每次 tokens | 费用 | 说明 |
|------|------|:----------:|----:|------|
| 合同分析（30k文档+答复） | moonshot-v1-32k | 30k+2k | ≈¥0.77 | |
| 长报告总结（100k文档） | moonshot-v1-128k | 100k+3k | ≈¥6.18 | |
| 普通对话 | moonshot-v1-8k | 500+500 | ≈¥0.012 | |

### 5.3 与竞品长文档对比

| 模型 | 上下文 | 长文档（100k）成本 |
|------|:------:|:-----------------:|
| moonshot-v1-128k | 128k | ≈¥6 |
| claude-3-5-sonnet（200k） | 200k | ≈¥2.2 |
| qwen-long | 10M | ≈¥0.1 |
| gemini-1.5-pro | 2M | ≈¥0.9 |

> 长文档场景：Qwen-Long 和 Gemini 更经济；Moonshot 适合中文 128k 场景且使用简便。

---

## 6. 特殊能力

| 能力 | 支持情况 |
|------|---------|
| 超长上下文 | ✅ 最高 128k |
| 文件上传（PDF/DOCX等）| ✅ Files API |
| 函数调用 | ✅ 所有模型 |
| 流式输出 | ✅ 所有模型 |
| 多模态（图片）| ❌ 目前不支持 |
| Embedding | ❌ 无 Embedding API |
| 推理增强 | ❌ 无推理模型 |
| 中文优化 | ✅ 月之暗面专项优化，口语中文表现好 |

---

## 7. 速率限制

| 模型 | RPM | 并发数 |
|------|----:|------:|
| moonshot-v1-8k | 60 | 3 |
| moonshot-v1-32k | 60 | 3 |
| moonshot-v1-128k | 60 | 3 |

> 默认并发较低（3），高并发场景需联系月之暗面申请提升。

---

## 8. MaaS 接入评估

### 8.1 接入配置

```yaml
vendor: moonshot
base_url: "https://api.moonshot.cn/v1"
auth_type: bearer_token
api_key_env: MOONSHOT_API_KEY
adapter: openai_compatible
timeout: 120s                  # 长文档响应需要更长超时
```

### 8.2 Adapter 复杂度：极低

完全 OpenAI 兼容，仅换 `base_url` 和 `api_key`，无需任何格式转换。

### 8.3 MaaS 路由策略建议

```yaml
# 长文档任务自动路由到 moonshot
routing_rules:
  - condition: "input_tokens >= 30000 AND language == 'zh'"
    route_to: moonshot-v1-128k
    fallback: qwen-long
```

### 8.4 注意事项

1. **并发上限低**：默认仅 3 个并发，对于高并发场景是瓶颈，需要提前申请提额或做请求队列
2. **无 Embedding**：需要向量检索时，路由至智谱 AI 或阿里云百炼
3. **无图片支持**：多模态请求需路由到其他模型
4. **文件 API 上传后 60 分钟过期**：不适合长期存储，每次需重新上传
5. **中文对话体验好**：特别适合面向 C 端的中文对话场景

---

## 9. 常见错误码

| 状态码 | 含义 | 处理建议 |
|:------:|-----|---------|
| 400 | 请求参数错误 | 检查 messages / model |
| 401 | 鉴权失败 | 检查 API Key |
| 429 | 超出速率限制 | 退避重试，降低并发 |
| 500 | 服务内部错误 | 重试 |
