# 深度求索（DeepSeek）供应商分析

**文档版本：** V1.0  
**最后更新：** 2026-05-20  
**官网：** https://www.deepseek.com  
**API 文档：** https://platform.deepseek.com/api-docs  
**定价页：** https://platform.deepseek.com/api-docs/pricing

---

## 1. 基本信息

| 项目 | 内容 |
|------|------|
| 公司 | 深度求索（杭州深度求索人工智能基础技术研究有限公司） |
| 国内可访问 | ✅ 直连可用 |
| API 格式 | **OpenAI 完全兼容**（最高兼容性之一） |
| 开源情况 | ✅ 模型权重开源（DeepSeek-V3/R1 均在 HuggingFace 开放） |
| 计费单位 | per 1M tokens |
| 结算货币 | CNY（人民币） |
| 支付方式 | 微信/支付宝充值（预付）|

---

## 2. API 接入

### 2.1 认证

```http
Authorization: Bearer sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
Content-Type: application/json
```

### 2.2 Base URL

```
https://api.deepseek.com/v1
```

**Chat Completions：**
```
POST https://api.deepseek.com/v1/chat/completions
```

### 2.3 SDK

直接使用 OpenAI Python SDK，仅修改 `base_url`：

```python
from openai import OpenAI

client = OpenAI(
    api_key="sk-xxx",
    base_url="https://api.deepseek.com/v1"
)

response = client.chat.completions.create(
    model="deepseek-chat",
    messages=[{"role": "user", "content": "你好"}]
)
```

---

## 3. 调用格式

### 3.1 标准请求（与 OpenAI 完全一致）

```json
{
  "model": "deepseek-chat",
  "messages": [
    {"role": "system", "content": "你是一个有帮助的助手"},
    {"role": "user",   "content": "介绍一下深度学习"}
  ],
  "temperature": 0.7,
  "max_tokens": 4096,
  "stream": false,
  "top_p": 1.0,
  "frequency_penalty": 0,
  "presence_penalty": 0
}
```

### 3.2 响应格式（与 OpenAI 完全一致）

```json
{
  "id": "chatcmpl-abc123",
  "object": "chat.completion",
  "created": 1716200000,
  "model": "deepseek-chat",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "深度学习是机器学习的一个分支..."
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 25,
    "completion_tokens": 150,
    "total_tokens": 175,
    "prompt_cache_hit_tokens": 0,
    "prompt_cache_miss_tokens": 25
  }
}
```

### 3.3 推理模型（DeepSeek-Reasoner / R1）特殊字段

R1 模型会在回复中携带思维链（Chain of Thought）：

```json
{
  "model": "deepseek-reasoner",
  "messages": [{"role": "user", "content": "证明勾股定理"}],
  "stream": false
}
```

响应中 `message` 包含额外字段：
```json
{
  "message": {
    "role": "assistant",
    "content": "勾股定理的证明如下：...",           // 最终答案
    "reasoning_content": "让我思考一下...\n首先考虑直角三角形..."  // 思维链（仅 R1）
  }
}
```

> 流式时 `reasoning_content` 通过 `delta.reasoning_content` 字段流出，与 `delta.content` 分开。

### 3.4 Function Calling

```json
{
  "model": "deepseek-chat",
  "messages": [{"role": "user", "content": "北京天气？"}],
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "get_weather",
        "description": "获取城市天气",
        "parameters": {
          "type": "object",
          "properties": {
            "city": {"type": "string"}
          },
          "required": ["city"]
        }
      }
    }
  ],
  "tool_choice": "auto"
}
```

---

## 4. 模型列表

### 4.1 主力模型

| 模型 ID | 上下文 | 输入 ¥/M | 输出 ¥/M | 特点 |
|---------|-------|--------:|--------:|------|
| `deepseek-chat` | 64k | 1（未命中缓存）| 2 | DeepSeek-V3 最新版，旗舰对话 |
| `deepseek-chat`（缓存命中）| 64k | **0.1** | 2 | Prompt 缓存命中时极低价 |
| `deepseek-reasoner` | 64k | 4（未命中）| 16 | DeepSeek-R1 推理模型 |
| `deepseek-reasoner`（缓存命中）| 64k | **0.5** | 16 | 推理模型缓存命中价 |

> **注意：** `deepseek-chat` 指向最新的 DeepSeek-V3 模型；`deepseek-reasoner` 指向最新 R1 模型。

### 4.2 历史版本（可指定）

| 模型 ID | 说明 |
|---------|------|
| `deepseek-v3` | V3 原始版本 |
| `deepseek-r1` | R1 原始版本 |
| `deepseek-v2-5` | 上一代旗舰 |
| `deepseek-coder-v2` | 代码专用版（支持 Function Calling） |

---

## 5. 价格分析

### 5.1 Prompt Caching 机制

DeepSeek 的缓存命中规则：
- **缓存命中**：相同 prompt 前缀在 **1小时** 内再次请求时命中
- **命中价格**：输入仅 ¥0.1/M（相比标准 ¥1/M 节省 **90%**）
- **适用场景**：固定长 System Prompt + 不同用户问题

### 5.2 与主流模型价格对比

| 模型 | 输入 ¥/M | 输出 ¥/M | 性能参考 |
|------|--------:|--------:|---------|
| **deepseek-chat（缓存命中）** | **0.1** | **2** | 接近 GPT-4o 70-80% 水平 |
| deepseek-chat | 1 | 2 | |
| gpt-4o-mini | ≈1.1 | ≈4.4 | 参考基准 |
| gpt-4o | ≈36 | ≈109 | 参考旗舰 |
| qwen-max | 40 | 120 | 国产旗舰 |
| **deepseek-reasoner** | **4** | **16** | 推理能力接近 o1 |
| o1 | ≈109 | ≈435 | OpenAI 推理旗舰 |

**结论：DeepSeek-chat 是当前性价比最高的模型之一，推理模型 R1 相比 o1 便宜约 97%。**

### 5.3 成本估算

| 场景 | 模型 | 每次 Tokens | 每次费用 | 10万次/月 |
|------|------|:----------:|-------:|----------:|
| 普通对话 | deepseek-chat | 500+200 | ≈¥0.0009 | ≈¥90 |
| 缓存命中场景 | deepseek-chat（hit） | 500+200 | ≈¥0.00045 | ≈¥45 |
| 代码生成 | deepseek-chat | 1k+500 | ≈¥0.002 | ≈¥200 |
| 复杂推理 | deepseek-reasoner | 1k+2k | ≈¥0.036 | ≈¥3,600 |

---

## 6. 特殊能力

| 能力 | 支持情况 |
|------|---------|
| 函数调用 | ✅ deepseek-chat 支持 |
| 流式输出 | ✅ 所有模型 |
| 思维链（CoT） | ✅ deepseek-reasoner（R1）原生输出 |
| 多模态（图片） | ❌ 目前不支持 |
| Embedding | ❌ 无官方 Embedding API |
| Prompt Caching | ✅ 1小时有效，命中减价 90% |
| 前缀填充（FIM）| ✅ deepseek-chat 支持（代码补全场景） |
| 精调 | ⚠️ 部分支持，见官方文档 |
| 开源部署 | ✅ 模型权重公开，可自部署 |

---

## 7. 速率限制

| 限制类型 | 默认值 |
|---------|:------:|
| RPM（请求/分钟） | 60 |
| TPM（Tokens/分钟） | 500,000 |
| 并发请求数 | 无明确限制（实际看服务状态） |

> 可在 DeepSeek 控制台申请提高限额；高流量客户可联系商务。

---

## 8. MaaS 接入评估

### 8.1 接入配置

```yaml
vendor: deepseek
base_url: "https://api.deepseek.com/v1"
auth_type: bearer_token
api_key_env: DEEPSEEK_API_KEY
adapter: openai_compatible    # 完全兼容，直接复用 OpenAI Adapter
timeout: 120s                  # R1 推理可能很慢，需要长超时
```

### 8.2 Adapter 复杂度：极低

- 请求/响应格式与 OpenAI 完全一致
- 仅换 `base_url` 和 `api_key` 即可
- **唯一额外处理**：R1 的 `reasoning_content` 字段，MaaS 可选择性地：
  - 直接透传给调用方
  - 在内部记录（用于调试/可观测性）
  - 截断后只返回 `content`

### 8.3 R1 思维链透传示例

```python
def normalize_deepseek_response(resp, include_reasoning=False):
    msg = resp.choices[0].message
    result = {
        "role": "assistant",
        "content": msg.content
    }
    if include_reasoning and hasattr(msg, 'reasoning_content'):
        result["reasoning_content"] = msg.reasoning_content  # 透传给调用方
    return result
```

### 8.4 注意事项

1. **R1 超时设置**：复杂推理任务可能耗时 30-120 秒，需适当加长超时
2. **R1 的 temperature**：推理模型建议 `temperature=1`（官方推荐），设为 0 效果反而差
3. **没有 Embedding**：如需向量能力，需路由至其他厂商（如智谱/阿里）
4. **没有视觉能力**：多模态请求需路由至 GPT-4o / Claude / qwen-vl
5. **缓存窗口**：相同前缀请求建议在 1 小时内完成，超时缓存失效重新计费
6. **开源替代方案**：如成本敏感，可评估自部署 DeepSeek 模型（需要 GPU 集群）

---

## 9. 常见错误码

| 错误码 | 含义 | 处理建议 |
|:------:|-----|---------|
| 400 | 请求参数错误 | 检查格式 |
| 401 | API Key 无效 | 检查 Key |
| 402 | 账户余额不足 | 充值 |
| 422 | 参数验证失败 | 检查 model / messages |
| 429 | 超出速率限制 | 退避重试 |
| 500 | 服务内部错误 | 重试 |
| 503 | 服务过载 | 退避重试 |
