# Meta Llama（开源模型）供应商分析

**文档版本：** V1.0  
**最后更新：** 2026-05-20  
**Meta 官方：** https://llama.meta.com  
**国内推荐托管：** 硅基流动（SiliconFlow）https://siliconflow.cn

---

## 1. 基本信息

| 项目 | 内容 |
|------|------|
| 公司 | Meta AI |
| 模型性质 | **开源**（权重公开，可自部署） |
| 国内可访问 | ⚠️ Meta 官方不可直连，国内托管平台可用 |
| API 格式 | **OpenAI 兼容**（依托管平台） |
| 核心优势 | 开源免费（自部署）、无数据泄露风险、丰富生态 |
| 计费方式 | 自部署：零 API 费用（只有算力成本）；托管平台：按 Token 付费 |

---

## 2. 托管平台选择

Llama 是开源模型，实际 API 调用需通过托管平台或自部署。

### 2.1 国内托管平台（可直连）

| 平台 | Base URL | 特点 |
|------|---------|------|
| **硅基流动（SiliconFlow）** | `https://api.siliconflow.cn/v1` | 国内最全，价格低，推荐 |
| 字节豆包（ARK） | `https://ark.cn-beijing.volces.com/api/v3` | 接入 llama-3-70b |
| 阿里云百炼 | `https://dashscope.aliyuncs.com/compatible-mode/v1` | 接入部分版本 |
| Cloudflare AI | `https://api.cloudflare.com/client/v4/accounts/.../ai/v1` | 小量免费 |

### 2.2 海外托管平台（需代理）

| 平台 | Base URL | 特点 |
|------|---------|------|
| Together AI | `https://api.together.xyz/v1` | 最全，价格合理 |
| Fireworks AI | `https://api.fireworks.ai/inference/v1` | 快速低延迟 |
| Groq | `https://api.groq.com/openai/v1` | **极速**（专用推理芯片） |
| Replicate | `https://api.replicate.com/v1` | 按需启动，灵活 |

---

## 3. API 接入（以硅基流动为例）

### 3.1 认证

```http
Authorization: Bearer sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
Content-Type: application/json
```

API Key 在硅基流动控制台创建，前缀为 `sk-`。

### 3.2 Base URL

```
https://api.siliconflow.cn/v1
```

### 3.3 SDK

```python
from openai import OpenAI

client = OpenAI(
    api_key="sk-xxx",
    base_url="https://api.siliconflow.cn/v1"
)

response = client.chat.completions.create(
    model="meta-llama/Llama-3.3-70B-Instruct",
    messages=[{"role": "user", "content": "你好"}]
)
```

---

## 4. 调用格式

格式与 OpenAI 完全一致（通过硅基流动/Together 等平台）：

### 4.1 标准 Chat 请求

```json
{
  "model": "meta-llama/Llama-3.3-70B-Instruct",
  "messages": [
    {"role": "system", "content": "你是一个有帮助的助手"},
    {"role": "user",   "content": "介绍一下开源大模型的发展"}
  ],
  "temperature": 0.7,
  "max_tokens": 4096,
  "stream": false
}
```

### 4.2 Function Calling（Llama 3 支持）

```json
{
  "model": "meta-llama/Llama-3.3-70B-Instruct",
  "messages": [{"role": "user", "content": "查询北京天气"}],
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
          }
        }
      }
    }
  ]
}
```

---

## 5. 模型列表

### 5.1 Llama 4 系列（最新）

| 模型 | 参数量 | 上下文 | 托管价格 ¥/M（参考） | 特点 |
|------|:-----:|:-----:|:-----------------:|------|
| Llama-4-Scout | 17B（激活） | 10M | ≈0.5-1 | MoE架构，超长上下文 |
| Llama-4-Maverick | 17B（激活） | 1M | ≈1-3 | 多模态，视觉 |

### 5.2 Llama 3.3 / 3.2 系列（成熟稳定）

| 模型 ID | 参数量 | 上下文 | 硅基流动 ¥/M | 特点 |
|---------|:-----:|:-----:|:----------:|------|
| `meta-llama/Llama-3.3-70B-Instruct` | 70B | 128k | ≈4 | 旗舰，综合最强 |
| `meta-llama/Llama-3.1-70B-Instruct` | 70B | 128k | ≈4 | 上一代旗舰 |
| `meta-llama/Llama-3.2-11B-Vision-Instruct` | 11B | 128k | ≈1 | **视觉理解**，轻量 |
| `meta-llama/Llama-3.2-90B-Vision-Instruct` | 90B | 128k | ≈4 | 视觉旗舰 |
| `meta-llama/Llama-3.2-3B-Instruct` | 3B | 128k | ≈0.3 | 极轻量，边缘场景 |
| `meta-llama/Llama-3.2-1B-Instruct` | 1B | 128k | ≈0.1 | 最小模型 |
| `meta-llama/Llama-3.1-8B-Instruct` | 8B | 128k | ≈1 | 轻量均衡 |

### 5.3 代码专用：CodeLlama

| 模型 ID | 参数量 | 特点 |
|---------|:-----:|------|
| `codellama/CodeLlama-34b-Instruct-hf` | 34B | 代码生成 |
| `codellama/CodeLlama-13b-Instruct-hf` | 13B | 轻量代码 |

### 5.4 自部署参考（GPU 要求）

| 模型 | 精度 | 显存需求 | 推荐 GPU |
|------|:---:|:-------:|---------|
| Llama-3.1-8B | FP16 | ~16GB | 1x A10 |
| Llama-3.1-8B | INT4 量化 | ~6GB | 1x 3090 |
| Llama-3.3-70B | FP16 | ~140GB | 4x A100 |
| Llama-3.3-70B | INT4 量化 | ~40GB | 2x A100 |

---

## 6. 价格分析

### 6.1 硅基流动托管价格（参考）

| 模型 | 输入 ¥/M | 输出 ¥/M | 说明 |
|------|--------:|--------:|------|
| Llama-3.3-70B-Instruct | 4 | 4 | 对称定价 |
| Llama-3.1-8B-Instruct | 1 | 1 | |
| Llama-3.2-3B-Instruct | 0.3 | 0.3 | |
| Llama-3.2-11B-Vision | 1 | 1 | 视觉 |

### 6.2 自部署 vs 托管成本对比

**假设：月调用量 1 亿 tokens（~10 亿 tokens/年）**

| 方案 | 月成本估算 | 适用场景 |
|------|:--------:|---------|
| 硅基流动（70B） | ≈¥400 | 小中量，快速上线 |
| 硅基流动（8B） | ≈¥100 | 小量，低成本 |
| 自部署 70B（A100×4） | ≈¥12,000（GPU 租用） | 大量，>10B tokens/月 |
| 自购 A100×4 服务器 | 摊销成本低，需初期投入 | 超大量，数据安全 |

> **结论：** 月调用量 < 10B tokens 时，托管平台更经济；超过后自部署有优势。

### 6.3 与闭源模型对比

| 模型 | 价格 ¥/M | 性能参考 |
|------|:-------:|---------|
| Llama-3.3-70B（托管） | ≈4 | 接近 GPT-4o-mini |
| GPT-4o-mini | ≈1.1 | 参考 |
| DeepSeek-V3 | 1 | 超过 Llama-70B |

> 就 API 调用价格而言，Llama 托管并无太大优势，主要优势在于**数据安全（自部署）**和**无审查**。

---

## 7. 特殊能力

| 能力 | 支持情况 |
|------|---------|
| 多模态（图片理解） | ✅ Llama-3.2-Vision 系列 |
| 函数调用 | ✅ Llama 3.x Instruct |
| 流式输出 | ✅ 所有模型（via 托管平台） |
| Embedding | ❌ 无官方 Embedding（可用 BAAI/bge 替代） |
| 推理增强 | ⚠️ 有限（无专用 reasoning 版本） |
| 开源权重 | ✅ 可自部署，完全可控 |
| 数据安全 | ✅ 自部署时数据不出内网 |
| 微调 | ✅ 官方支持 SFT + RLHF |
| 量化压缩 | ✅ GGUF/AWQ/GPTQ 格式可用 |
| 商业使用 | ✅ Meta Llama 3 License（月活 7 亿以上需申请） |

---

## 8. MaaS 接入评估

### 8.1 推荐接入方案

**方案 A：通过硅基流动（国内稳定，推荐）**

```yaml
vendor: siliconflow
base_url: "https://api.siliconflow.cn/v1"
auth_type: bearer_token
api_key_env: SILICONFLOW_API_KEY
adapter: openai_compatible
timeout: 60s

model_mapping:
  "llama-3.3-70b":       "meta-llama/Llama-3.3-70B-Instruct"
  "llama-3.1-8b":        "meta-llama/Llama-3.1-8B-Instruct"
  "llama-3.2-vision-11b": "meta-llama/Llama-3.2-11B-Vision-Instruct"
```

**方案 B：自部署（数据安全优先）**

```yaml
vendor: self_hosted_llama
base_url: "http://internal-llm-service:8000/v1"   # 内网地址
auth_type: bearer_token
api_key_env: INTERNAL_API_KEY
adapter: openai_compatible          # vLLM/Ollama 均提供 OpenAI 兼容接口
timeout: 60s
```

自部署推荐推理框架：
- **vLLM**：生产级，高吞吐，OpenAI 兼容
- **Ollama**：开发测试，简单易用
- **LMDeploy**：国内优化，支持 TurboMind 加速

### 8.2 注意事项

1. **模型 ID 格式**：硅基流动等平台使用 HuggingFace 格式 `org/model-name`，需在 MaaS 做映射
2. **Llama 3 Tokenizer**：与 GPT 不同，Token 数量可能差异较大（中文 Token 效率较低）
3. **中文能力**：Llama 原版中文能力一般，建议使用经过中文继续预训练的版本（如 Qwen、Yi 等）
4. **License 限制**：Llama 3 License 要求月活超 7 亿的产品需单独申请授权
5. **内容安全**：开源模型无内置安全过滤（与 OpenAI 不同），生产环境需自建内容审核
6. **硅基流动稳定性**：国内托管平台，稳定性和 SLA 不如 OpenAI，建议设置 Fallback

---

## 9. 开源生态补充

### 9.1 其他值得关注的开源模型（可通过相同渠道接入）

| 模型 | 厂商 | 特点 | 硅基流动支持 |
|------|------|------|:----------:|
| Qwen2.5-72B | 阿里 | 综合最强开源 | ✅ |
| DeepSeek-V3 | 深度求索 | 极低价开源旗舰 | ✅ |
| Yi-34B | 零一万物 | 中文优化开源 | ✅ |
| Mistral-7B | Mistral AI | 欧洲开源，轻量 | ✅ |
| Gemma-2-9B | Google | 开源 Gemma | ✅ |
| Phi-3.5-mini | Microsoft | 极小但强的模型 | ✅ |

### 9.2 推荐的 Embedding 开源模型

| 模型 | 维度 | 硅基流动价格 | 特点 |
|------|:---:|:----------:|------|
| `BAAI/bge-m3` | 1024 | ≈0 | 多语言，最推荐 |
| `BAAI/bge-large-zh-v1.5` | 1024 | ≈0 | 中文最强 |
| `jinaai/jina-embeddings-v3` | 1024 | ≈0 | 多语言高精度 |
