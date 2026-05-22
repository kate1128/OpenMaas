# MaaS平台 Cookbook：实战案例集

**文档版本：** V1.0  
**编写日期：** 2026年05月14日  
**适用读者：** 已完成基础接入的开发者，希望实现具体业务场景  
**密级：** 对外公开

---

## 目录

| # | 场景 | 难度 | 核心技术 |
|---|------|------|---------|
| 01 | [智能客服机器人](#01-智能客服机器人) | ⭐⭐ | 多轮对话 + 流式输出 |
| 02 | [RAG 知识库问答](#02-rag-知识库问答) | ⭐⭐⭐ | Embeddings + 向量检索 |
| 03 | [代码审查助手](#03-代码审查助手) | ⭐⭐ | Function Calling + 结构化输出 |
| 04 | [长文档摘要管道](#04-长文档摘要管道) | ⭐⭐⭐ | 文本分块 + 并发调用 |
| 05 | [SQL 生成器](#05-sql-生成器) | ⭐⭐ | 少样本提示 + 安全校验 |
| 06 | [多模态图片理解](#06-多模态图片理解) | ⭐⭐ | Base64 图片输入 |
| 07 | [AI Agent 工作流](#07-ai-agent-工作流) | ⭐⭐⭐⭐ | Tool Use + 循环推理 |
| 08 | [语义搜索引擎](#08-语义搜索引擎) | ⭐⭐⭐ | 批量 Embedding + 余弦相似度 |
| 09 | [实时内容安全过滤](#09-实时内容安全过滤) | ⭐⭐ | 分类 Prompt + 拦截逻辑 |
| 10 | [成本感知的自适应路由](#10-成本感知的自适应路由) | ⭐⭐⭐ | 动态模型选择 + 降本策略 |

---

## 01 智能客服机器人

**场景：** 电商/SaaS 产品的在线客服，支持多轮对话，记忆上下文，流式逐字输出。

```python
import os
from openai import OpenAI
from typing import Generator

client = OpenAI(
    api_key=os.environ["MAAS_API_KEY"],
    base_url="https://api.maas-platform.com/v1"
)

SYSTEM_PROMPT = """你是专业的客服助手。
规则：
1. 态度友好，称呼用户为"您"
2. 回答聚焦产品问题，无法回答时引导用户联系人工
3. 回答简洁，不超过200字
4. 遇到退款/投诉等敏感问题，优先安抚情绪"""

class CustomerServiceBot:
    def __init__(self, max_history_rounds: int = 8):
        self.history = [{"role": "system", "content": SYSTEM_PROMPT}]
        self.max_history_rounds = max_history_rounds

    def chat_stream(self, user_message: str) -> Generator[str, None, None]:
        self.history.append({"role": "user", "content": user_message})

        # 历史截断：只保留最近 N 轮，避免 Token 过多
        system = self.history[:1]
        dialogs = self.history[1:]
        if len(dialogs) > self.max_history_rounds * 2:
            dialogs = dialogs[-(self.max_history_rounds * 2):]
        messages = system + dialogs

        stream = client.chat.completions.create(
            model="qwen-turbo",          # 中文客服首选，成本低
            messages=messages,
            max_tokens=300,
            temperature=0.7,
            stream=True,
        )

        full_reply = ""
        for chunk in stream:
            delta = chunk.choices[0].delta.content or ""
            full_reply += delta
            yield delta                   # 逐字流式返回给前端

        self.history.append({"role": "assistant", "content": full_reply})

# 使用示例
bot = CustomerServiceBot()
print("客服机器人（输入 exit 退出）：")
while True:
    user_input = input("用户: ")
    if user_input.lower() == "exit":
        break
    print("客服: ", end="")
    for token in bot.chat_stream(user_input):
        print(token, end="", flush=True)
    print()
```

**关键设计点：**
- 历史截断防止 Token 超限，保留 System Prompt + 最近 N 轮
- 流式输出提升用户感知响应速度
- `qwen-turbo` 成本约为 GPT-4o 的 1/10，客服场景足够用

---

## 02 RAG 知识库问答

**场景：** 基于公司内部文档（PDF/Word/Markdown）的问答系统。

```python
import os
import json
from openai import OpenAI
import numpy as np
from typing import Optional

client = OpenAI(
    api_key=os.environ["MAAS_API_KEY"],
    base_url="https://api.maas-platform.com/v1"
)

# ── Step 1: 文档向量化（建库阶段，离线执行）──────────────────────────────

def embed_texts(texts: list[str]) -> list[list[float]]:
    """批量向量化文本，每批最多 100 条"""
    all_embeddings = []
    batch_size = 100
    for i in range(0, len(texts), batch_size):
        batch = texts[i:i + batch_size]
        response = client.embeddings.create(
            model="text-embedding-3-small",
            input=batch
        )
        all_embeddings.extend([d.embedding for d in response.data])
    return all_embeddings

def build_knowledge_base(docs: list[dict]) -> list[dict]:
    """
    docs: [{"id": "1", "title": "...", "content": "..."}]
    返回: 带 embedding 的文档列表
    """
    texts = [f"{d['title']}\n{d['content']}" for d in docs]
    embeddings = embed_texts(texts)
    for doc, emb in zip(docs, embeddings):
        doc["embedding"] = emb
    return docs


# ── Step 2: 语义检索（运行时）──────────────────────────────────────────────

def cosine_similarity(a: list[float], b: list[float]) -> float:
    a, b = np.array(a), np.array(b)
    return float(np.dot(a, b) / (np.linalg.norm(a) * np.linalg.norm(b)))

def retrieve(query: str, knowledge_base: list[dict], top_k: int = 3) -> list[dict]:
    """检索与 query 最相关的 top_k 个文档片段"""
    query_emb = embed_texts([query])[0]
    scored = [
        {**doc, "score": cosine_similarity(query_emb, doc["embedding"])}
        for doc in knowledge_base
    ]
    return sorted(scored, key=lambda x: x["score"], reverse=True)[:top_k]


# ── Step 3: 增强生成（RAG 问答）─────────────────────────────────────────────

def rag_answer(query: str, knowledge_base: list[dict]) -> str:
    # 1. 检索相关文档
    contexts = retrieve(query, knowledge_base, top_k=3)

    # 2. 构建 Prompt，注入检索到的上下文
    context_text = "\n\n---\n\n".join([
        f"【{c['title']}】\n{c['content']}"
        for c in contexts if c["score"] > 0.6  # 过滤低相关性
    ])

    if not context_text:
        return "抱歉，我在知识库中没有找到相关信息，请联系管理员。"

    prompt = f"""请根据以下知识库内容回答用户问题。
如果知识库中没有相关信息，请明确说明"知识库中没有相关内容"，不要编造答案。

知识库内容：
{context_text}

用户问题：{query}

请用简洁、准确的语言回答："""

    response = client.chat.completions.create(
        model="qwen-plus",  # 中等复杂度，平衡性价比
        messages=[{"role": "user", "content": prompt}],
        max_tokens=500,
        temperature=0.3     # 降低随机性，提高准确性
    )
    return response.choices[0].message.content


# 示例使用
sample_docs = [
    {"id": "1", "title": "退款政策", "content": "购买后7天内可申请无理由退款..."},
    {"id": "2", "title": "配送说明", "content": "标准配送3-5个工作日，顺丰次日达..."},
]
kb = build_knowledge_base(sample_docs)
print(rag_answer("我可以退货吗？", kb))
```

---

## 03 代码审查助手

**场景：** CI/CD 流水线中的自动代码审查，输出结构化 JSON 报告。

```python
import json
from openai import OpenAI

client = OpenAI(
    api_key=os.environ["MAAS_API_KEY"],
    base_url="https://api.maas-platform.com/v1"
)

CODE_REVIEW_PROMPT = """你是一位资深代码审查工程师。
请对给定的代码片段进行审查，以 JSON 格式返回审查结果。

返回格式（严格 JSON，不要有额外文字）：
{
  "overall_score": 1-10的整数评分,
  "summary": "总体评价一句话",
  "issues": [
    {
      "type": "bug|security|performance|style",
      "severity": "critical|high|medium|low",
      "line": 行号或null,
      "description": "问题描述",
      "suggestion": "改进建议"
    }
  ],
  "positives": ["做得好的地方"]
}"""

def review_code(code: str, language: str = "python") -> dict:
    response = client.chat.completions.create(
        model="gpt-4o",          # 代码审查用强模型保证准确性
        messages=[
            {"role": "system", "content": CODE_REVIEW_PROMPT},
            {"role": "user", "content": f"请审查以下 {language} 代码：\n\n```{language}\n{code}\n```"}
        ],
        response_format={"type": "json_object"},  # 强制 JSON 输出
        temperature=0.2,
        max_tokens=2000
    )

    result = json.loads(response.choices[0].message.content)
    return result

# 示例
sample_code = """
def get_user(user_id):
    query = f"SELECT * FROM users WHERE id = {user_id}"  # SQL注入！
    return db.execute(query)
"""

review = review_code(sample_code)
print(f"评分: {review['overall_score']}/10")
for issue in review['issues']:
    print(f"[{issue['severity'].upper()}] {issue['description']}")
```

---

## 04 长文档摘要管道

**场景：** 处理超长文档（超过单次上下文限制），通过分块+并发+汇总实现完整摘要。

```python
import asyncio
from openai import AsyncOpenAI

async_client = AsyncOpenAI(
    api_key=os.environ["MAAS_API_KEY"],
    base_url="https://api.maas-platform.com/v1"
)

def split_text(text: str, chunk_size: int = 3000, overlap: int = 200) -> list[str]:
    """将长文本按字符数分块，带重叠避免语义断裂"""
    chunks = []
    start = 0
    while start < len(text):
        end = start + chunk_size
        chunks.append(text[start:end])
        start = end - overlap
    return chunks

async def summarize_chunk(chunk: str, chunk_index: int) -> str:
    response = await async_client.chat.completions.create(
        model="claude-3-5-haiku",   # 长文本用 Claude（200K 窗口，价格适中）
        messages=[{
            "role": "user",
            "content": f"请对以下文本片段（第{chunk_index+1}段）进行简洁摘要，100字以内：\n\n{chunk}"
        }],
        max_tokens=200
    )
    return response.choices[0].message.content

async def summarize_long_document(document: str) -> str:
    chunks = split_text(document)
    print(f"文档共分为 {len(chunks)} 段，开始并发摘要...")

    # 并发摘要所有分块（最多10个并发）
    semaphore = asyncio.Semaphore(10)
    async def bounded_summarize(chunk, idx):
        async with semaphore:
            return await summarize_chunk(chunk, idx)

    chunk_summaries = await asyncio.gather(*[
        bounded_summarize(chunk, i) for i, chunk in enumerate(chunks)
    ])

    # 汇总所有分块摘要，生成最终摘要
    combined = "\n\n".join([f"第{i+1}段：{s}" for i, s in enumerate(chunk_summaries)])
    final_response = await async_client.chat.completions.create(
        model="qwen-plus",
        messages=[{
            "role": "user",
            "content": f"以下是一篇长文档各段的摘要，请综合生成全文摘要（300字以内）：\n\n{combined}"
        }],
        max_tokens=400
    )
    return final_response.choices[0].message.content

# 使用示例
long_doc = "..." * 10000  # 很长的文档
summary = asyncio.run(summarize_long_document(long_doc))
print(summary)
```

---

## 05 SQL 生成器

**场景：** 将自然语言查询转换为 SQL，并进行安全校验防止危险操作。

```python
import re

SCHEMA = """
数据库 Schema：
- users(id, name, email, created_at, status)
- orders(id, user_id, amount, status, created_at)  
- products(id, name, price, stock, category)
- order_items(id, order_id, product_id, quantity, unit_price)
"""

FORBIDDEN_PATTERNS = [
    r'\bDROP\b', r'\bDELETE\b', r'\bTRUNCATE\b',
    r'\bUPDATE\b', r'\bINSERT\b', r'\bALTER\b'
]

def text_to_sql(natural_query: str) -> dict:
    prompt = f"""{SCHEMA}

请将以下自然语言转换为 SQL 查询语句。
规则：
1. 只生成 SELECT 语句，禁止任何修改数据的操作
2. 返回 JSON 格式：{{"sql": "...", "explanation": "..."}}
3. 使用参数化占位符替代具体值（如 :user_id）

用户查询：{natural_query}"""

    response = client.chat.completions.create(
        model="deepseek-coder-v2",   # 代码/SQL 专用模型
        messages=[{"role": "user", "content": prompt}],
        response_format={"type": "json_object"},
        temperature=0
    )

    result = json.loads(response.choices[0].message.content)
    sql = result.get("sql", "")

    # 安全校验：拦截危险操作
    for pattern in FORBIDDEN_PATTERNS:
        if re.search(pattern, sql, re.IGNORECASE):
            return {"error": f"生成的 SQL 包含危险操作，已拦截", "sql": sql}

    return result

# 示例
result = text_to_sql("查询上个月订单金额超过1000元的用户名单")
print(result["sql"])
# SELECT DISTINCT u.name, u.email
# FROM users u
# JOIN orders o ON u.id = o.user_id
# WHERE o.amount > 1000
#   AND o.created_at BETWEEN DATE_TRUNC('month', NOW() - INTERVAL '1 month')
#   AND DATE_TRUNC('month', NOW())
```

---

## 06 多模态图片理解

**场景：** 分析上传的产品图片，自动生成商品描述。

```python
import base64
from pathlib import Path

def encode_image(image_path: str) -> str:
    """将图片转为 Base64"""
    with open(image_path, "rb") as f:
        return base64.b64encode(f.read()).decode("utf-8")

def analyze_product_image(image_path: str) -> dict:
    base64_image = encode_image(image_path)
    ext = Path(image_path).suffix.lstrip(".")
    media_type = f"image/{ext if ext != 'jpg' else 'jpeg'}"

    response = client.chat.completions.create(
        model="gpt-4o",     # 多模态仅 GPT-4o 支持
        messages=[{
            "role": "user",
            "content": [
                {
                    "type": "image_url",
                    "image_url": {
                        "url": f"data:{media_type};base64,{base64_image}",
                        "detail": "high"
                    }
                },
                {
                    "type": "text",
                    "text": """请分析这张产品图片，返回 JSON：
{
  "product_name": "推断的商品名",
  "category": "商品类别",
  "description": "100字商品描述",
  "key_features": ["特点1", "特点2", "特点3"],
  "suggested_tags": ["标签1", "标签2"]
}"""
                }
            ]
        }],
        response_format={"type": "json_object"},
        max_tokens=500
    )

    return json.loads(response.choices[0].message.content)

# result = analyze_product_image("./product.jpg")
```

---

## 07 AI Agent 工作流

**场景：** 能够自主使用工具完成任务的 AI Agent（搜索 + 计算 + 汇总）。

```python
import json
from typing import Any

# 定义工具
tools = [
    {
        "type": "function",
        "function": {
            "name": "search_knowledge_base",
            "description": "搜索内部知识库",
            "parameters": {
                "type": "object",
                "properties": {
                    "query": {"type": "string", "description": "搜索关键词"}
                },
                "required": ["query"]
            }
        }
    },
    {
        "type": "function",
        "function": {
            "name": "calculate",
            "description": "执行数学计算",
            "parameters": {
                "type": "object",
                "properties": {
                    "expression": {"type": "string", "description": "数学表达式，如 '100 * 0.85'"}
                },
                "required": ["expression"]
            }
        }
    },
    {
        "type": "function",
        "function": {
            "name": "send_email",
            "description": "发送邮件",
            "parameters": {
                "type": "object",
                "properties": {
                    "to": {"type": "string"},
                    "subject": {"type": "string"},
                    "body": {"type": "string"}
                },
                "required": ["to", "subject", "body"]
            }
        }
    }
]

# 工具实现
def execute_tool(name: str, args: dict) -> Any:
    if name == "search_knowledge_base":
        return f"找到关于 '{args['query']}' 的相关信息：..."  # 实际接入向量库
    elif name == "calculate":
        return str(eval(args["expression"]))  # 注意：生产环境用安全的解析器
    elif name == "send_email":
        return f"邮件已发送至 {args['to']}"  # 实际接入 SMTP

# Agent 主循环
def run_agent(user_request: str, max_steps: int = 10) -> str:
    messages = [
        {"role": "system", "content": "你是一个能够使用工具完成任务的 AI 助手。请逐步思考并使用合适的工具。"},
        {"role": "user", "content": user_request}
    ]

    for step in range(max_steps):
        response = client.chat.completions.create(
            model="gpt-4o",
            messages=messages,
            tools=tools,
            tool_choice="auto"
        )

        msg = response.choices[0].message
        messages.append({"role": "assistant", "content": msg.content, "tool_calls": msg.tool_calls})

        # 没有工具调用，说明任务完成
        if not msg.tool_calls:
            return msg.content

        # 执行所有工具调用
        for tool_call in msg.tool_calls:
            tool_name = tool_call.function.name
            tool_args = json.loads(tool_call.function.arguments)
            tool_result = execute_tool(tool_name, tool_args)

            messages.append({
                "role": "tool",
                "tool_call_id": tool_call.id,
                "content": str(tool_result)
            })

    return "任务未能在最大步骤内完成"

# 使用示例
result = run_agent("查询今年的销售数据，计算同比增长率，并发邮件给 ceo@company.com")
print(result)
```

---

## 08 语义搜索引擎

**场景：** 为产品内容库构建语义搜索，支持"类似商品"推荐。

```python
import numpy as np
from dataclasses import dataclass

@dataclass
class SearchResult:
    id: str
    title: str
    score: float
    snippet: str

class SemanticSearchEngine:
    def __init__(self):
        self.index = []  # [(id, title, content, embedding)]

    def add_documents(self, docs: list[dict]):
        """批量添加文档到索引"""
        texts = [f"{d['title']} {d['content']}" for d in docs]
        embeddings = self._batch_embed(texts)
        for doc, emb in zip(docs, embeddings):
            self.index.append((doc["id"], doc["title"], doc["content"], emb))
        print(f"已添加 {len(docs)} 条文档，索引总量：{len(self.index)}")

    def search(self, query: str, top_k: int = 5, threshold: float = 0.5) -> list[SearchResult]:
        if not self.index:
            return []
        query_emb = self._batch_embed([query])[0]
        results = []
        for doc_id, title, content, doc_emb in self.index:
            score = self._cosine_sim(query_emb, doc_emb)
            if score >= threshold:
                results.append(SearchResult(
                    id=doc_id,
                    title=title,
                    score=score,
                    snippet=content[:100] + "..."
                ))
        return sorted(results, key=lambda x: x.score, reverse=True)[:top_k]

    def _batch_embed(self, texts: list[str]) -> list[list[float]]:
        resp = client.embeddings.create(model="text-embedding-3-small", input=texts)
        return [d.embedding for d in sorted(resp.data, key=lambda x: x.index)]

    @staticmethod
    def _cosine_sim(a, b):
        a, b = np.array(a), np.array(b)
        return float(np.dot(a, b) / (np.linalg.norm(a) * np.linalg.norm(b)))


engine = SemanticSearchEngine()
engine.add_documents([
    {"id": "1", "title": "无线蓝牙耳机", "content": "主动降噪，30小时续航..."},
    {"id": "2", "title": "有线耳机", "content": "Hi-Fi音质，专业录音..."},
])
results = engine.search("降噪耳机推荐")
for r in results:
    print(f"{r.title} ({r.score:.2f}): {r.snippet}")
```

---

## 09 实时内容安全过滤

**场景：** 对用户输入和模型输出进行内容安全审核，防止违规内容。

```python
from enum import Enum

class SafetyLevel(Enum):
    SAFE = "safe"
    WARN = "warn"
    BLOCK = "block"

SAFETY_CHECK_PROMPT = """请对以下文本进行内容安全审核。
检测维度：色情、暴力、政治敏感、个人信息、欺诈诱导、其他违规。

返回 JSON（严格格式）：
{
  "level": "safe|warn|block",
  "categories": ["触发的类别，无则为空数组"],
  "reason": "简短说明，safe时为null"
}"""

def check_content(text: str) -> dict:
    """安全检查，优先快速返回"""
    response = client.chat.completions.create(
        model="qwen-turbo",    # 用快速模型，降低延迟
        messages=[
            {"role": "system", "content": SAFETY_CHECK_PROMPT},
            {"role": "user", "content": text[:2000]}  # 限制长度，节省 Token
        ],
        response_format={"type": "json_object"},
        temperature=0,
        max_tokens=100
    )
    return json.loads(response.choices[0].message.content)

def safe_chat(user_message: str) -> str:
    # 1. 检查用户输入
    input_check = check_content(user_message)
    if input_check["level"] == "block":
        return f"您的消息包含违规内容（{', '.join(input_check['categories'])}），无法处理。"

    # 2. 正常调用模型
    response = client.chat.completions.create(
        model="qwen-plus",
        messages=[{"role": "user", "content": user_message}],
        max_tokens=500
    )
    model_reply = response.choices[0].message.content

    # 3. 检查模型输出
    output_check = check_content(model_reply)
    if output_check["level"] == "block":
        return "抱歉，我无法回答这个问题。"

    return model_reply
```

---

## 10 成本感知的自适应路由

**场景：** 根据请求内容的复杂度，动态选择模型，在效果和成本之间取得最优平衡。

```python
import tiktoken

def estimate_complexity(message: str) -> str:
    """
    估算请求复杂度（simple / medium / complex）
    依据：消息长度 + 关键词检测
    """
    token_count = len(message.split())  # 简化版 token 估算

    complex_keywords = ["分析", "推理", "对比", "评估", "设计方案", "analyze", "reasoning"]
    code_keywords = ["代码", "函数", "实现", "code", "function", "implement", "debug"]

    has_complex = any(k in message for k in complex_keywords)
    has_code = any(k in message for k in code_keywords)

    if token_count > 500 or has_complex:
        return "complex"
    elif token_count > 100 or has_code:
        return "medium"
    else:
        return "simple"

MODEL_ROUTING = {
    "simple":  {"model": "qwen-turbo",         "cost_per_1k": 0.004},
    "medium":  {"model": "qwen-plus",           "cost_per_1k": 0.012},
    "complex": {"model": "gpt-4o",              "cost_per_1k": 0.04},
}

def adaptive_chat(user_message: str, force_economy: bool = False) -> dict:
    complexity = estimate_complexity(user_message)
    if force_economy and complexity == "complex":
        complexity = "medium"  # 强制经济模式降级

    routing = MODEL_ROUTING[complexity]
    response = client.chat.completions.create(
        model=routing["model"],
        messages=[{"role": "user", "content": user_message}],
        max_tokens=1000
    )

    tokens = response.usage.total_tokens
    estimated_cost = tokens / 1000 * routing["cost_per_1k"]

    return {
        "reply": response.choices[0].message.content,
        "model_used": routing["model"],
        "complexity": complexity,
        "tokens": tokens,
        "estimated_cost_cny": round(estimated_cost, 6)
    }

# 测试
r1 = adaptive_chat("你好")                          # → qwen-turbo
r2 = adaptive_chat("帮我分析这段代码的性能问题...")  # → gpt-4o
r3 = adaptive_chat("你好", force_economy=True)      # → qwen-turbo（强制）

for r in [r1, r2, r3]:
    print(f"[{r['complexity']}] {r['model_used']} | ¥{r['estimated_cost_cny']}")
```

---

## 附：Prompt 工程速查卡

```
┌────────────────────────────────────────────────────────────────┐
│                      Prompt 工程速查卡                          │
├─────────────────────────┬──────────────────────────────────────┤
│ 需要结构化输出           │ 在 Prompt 结尾加"返回JSON格式"         │
│                         │ 使用 response_format={"type":"json"}  │
├─────────────────────────┼──────────────────────────────────────┤
│ 需要一致/确定性输出      │ temperature=0                          │
├─────────────────────────┼──────────────────────────────────────┤
│ 需要创意/多样性输出      │ temperature=0.8~1.2                    │
├─────────────────────────┼──────────────────────────────────────┤
│ 需要遵循严格格式         │ 提供 1-3 个输出示例（Few-shot）         │
├─────────────────────────┼──────────────────────────────────────┤
│ 减少幻觉                 │ "只根据以下信息回答，如不知道请说不知道" │
├─────────────────────────┼──────────────────────────────────────┤
│ 控制回答长度             │ "请用X字以内回答" 比 max_tokens 更可靠  │
├─────────────────────────┼──────────────────────────────────────┤
│ 角色扮演                 │ System Prompt 定义角色更稳定           │
├─────────────────────────┼──────────────────────────────────────┤
│ 让模型推理更准确         │ "请一步步思考" / "先列出分析步骤"       │
└─────────────────────────┴──────────────────────────────────────┘
```
