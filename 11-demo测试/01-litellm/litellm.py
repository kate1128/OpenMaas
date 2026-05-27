"""
LiteLLM SDK 真实使用示例

运行前提：
  pip install litellm

每个示例都可独立运行。未设置真实 API Key 时会触发异常，替换为真实 Key 即可测试。
"""

import asyncio
import json
import os

# ============================================================
# 示例 1：基础调用 — 直接 acompletion
# ============================================================
async def example_basic_completion():
    import litellm

    response = await litellm.acompletion(
        model="openai/gpt-4o",
        messages=[{"role": "user", "content": "用一句话解释量子计算"}],
        api_key=os.getenv("OPENAI_API_KEY", "sk-placeholder"),
        temperature=0.7,
        max_tokens=200,
    )
    print(response.choices[0].message.content)
    print(f"token用量: {response.usage}")
    return response


# ============================================================
# 示例 2：Router 多部署负载均衡
# ============================================================
async def example_router_basic():
    from litellm import Router

    router = Router(
        model_list=[
            {
                "model_name": "gpt-4o",
                "litellm_params": {
                    "model": "openai/gpt-4o",
                    "api_key": os.getenv("OPENAI_API_KEY", "sk-placeholder"),
                    "rpm": 9000,
                },
            },
            {
                "model_name": "gpt-4o",
                "litellm_params": {
                    "model": "openai/gpt-4o",
                    "api_key": os.getenv("OPENAI_API_KEY_2", "sk-placeholder"),
                    "rpm": 1000,
                },
            },
        ],
        routing_strategy="simple-shuffle",
        enable_pre_call_checking=True,
        num_retries=0,
        context_length_manager="ljm",
    )

    response = await router.acompletion(
        model="gpt-4o",
        messages=[{"role": "user", "content": "Hello"}],
    )
    # 检查实际命中的是哪个 deployment
    print(f"命中的 deploy ID: {response._hidden_params.get('model_id')}")
    print(response.choices[0].message.content)


# ============================================================
# 示例 3：流式调用
# ============================================================
async def example_stream():
    import litellm

    stream = await litellm.acompletion(
        model="openai/gpt-4o",
        messages=[{"role": "user", "content": "从 1 数到 5"}],
        api_key=os.getenv("OPENAI_API_KEY", "sk-placeholder"),
        stream=True,
    )

    async for chunk in stream:
        delta = chunk.choices[0].delta.content or ""
        print(delta, end="", flush=True)
    print()


# ============================================================
# 示例 4：带 Fallback 的 Router
# ============================================================
async def example_fallback():
    from litellm import Router

    router = Router(
        model_list=[
            {
                "model_name": "gpt-4o",
                "litellm_params": {
                    "model": "openai/gpt-4o",
                    "api_key": "sk-bad-key",  # 故意错误，触发 fallback
                },
            },
            {
                "model_name": "gpt-4o-mini",
                "litellm_params": {
                    "model": "openai/gpt-4o-mini",
                    "api_key": os.getenv("OPENAI_API_KEY", "sk-placeholder"),
                },
            },
        ],
        num_retries=0,
        fallbacks=[{"gpt-4o": ["gpt-4o-mini"]}],
    )

    response = await router.acompletion(
        model="gpt-4o",
        messages=[{"role": "user", "content": "Hello"}],
    )
    print(f"实际响应模型: {response._hidden_params.get('model')}")
    print(response.choices[0].message.content)


# ============================================================
# 示例 5：自定义路由策略
# ============================================================
async def example_custom_strategy():
    from litellm import Router
    from litellm.router import CustomRoutingStrategyBase

    class TenantAwareStrategy(CustomRoutingStrategyBase):
        """
        根据请求 metadata 中的 tenant_id 选择不同后端。
        模拟场景：tenant-A 走 gpt-4o，其他走 gpt-4o-mini。
        """
        async def async_get_available_deployment(self, model, messages, **kwargs):
            model_list = kwargs.get("model_list", [])
            request_kwargs = kwargs.get("request_kwargs", {})
            metadata = request_kwargs.get("metadata", {})
            tenant_id = metadata.get("maas_tenant_id", "default")

            for deploy in model_list:
                model_id = deploy.get("model_info", {}).get("id", "")
                if tenant_id == "tenant-A" and "gpt-4o-deploy" in model_id:
                    return deploy
                if tenant_id != "tenant-A" and "mini-deploy" in model_id:
                    return deploy
            return model_list[0] if model_list else None

        def get_available_deployment(self, model, messages, **kwargs):
            return None  # 只实现异步版本

    router = Router(
        model_list=[
            {
                "model_name": "gpt-4o",
                "litellm_params": {
                    "model": "openai/gpt-4o",
                    "api_key": os.getenv("OPENAI_API_KEY", "sk-placeholder"),
                },
                "model_info": {"id": "gpt-4o-deploy"},
            },
            {
                "model_name": "gpt-4o",
                "litellm_params": {
                    "model": "openai/gpt-4o-mini",
                    "api_key": os.getenv("OPENAI_API_KEY", "sk-placeholder"),
                },
                "model_info": {"id": "mini-deploy"},
            },
        ],
        routing_strategy="custom",
    )
    router.set_custom_routing_strategy(TenantAwareStrategy())

    # tenant-A → gpt-4o
    resp_a = await router.acompletion(
        model="gpt-4o",
        messages=[{"role": "user", "content": "Hello"}],
        metadata={"maas_tenant_id": "tenant-A"},
    )
    print(f"tenant-A 命中: {resp_a._hidden_params.get('model_id')}")

    # 其他 → gpt-4o-mini
    resp_b = await router.acompletion(
        model="gpt-4o",
        messages=[{"role": "user", "content": "Hello"}],
        metadata={"maas_tenant_id": "tenant-B"},
    )
    print(f"tenant-B 命中: {resp_b._hidden_params.get('model_id')}")


# ============================================================
# 示例 6：错误映射与处理
# ============================================================
async def example_error_handling():
    import litellm

    try:
        await litellm.acompletion(
            model="openai/gpt-4o",
            messages=[{"role": "user", "content": "Hi"}],
            api_key="sk-invalid",
        )
    except litellm.AuthenticationError as e:
        print(f"认证错误 (可重试: 否): {e}")
    except litellm.RateLimitError as e:
        print(f"限流 (可重试: 是): {e}")
    except litellm.Timeout as e:
        print(f"超时 (可重试: 是): {e}")
    except litellm.ServiceUnavailableError as e:
        print(f"服务不可用 (可重试: 是): {e}")
    except litellm.ContextWindowExceededError as e:
        print(f"超出上下文 (可重试: 否): {e}")


# ============================================================
# 示例 7：Embedding 调用
# ============================================================
async def example_embedding():
    import litellm

    response = await litellm.aembedding(
        model="text-embedding-3-small",
        input=["LiteLLM 是什么"],
        api_key=os.getenv("OPENAI_API_KEY", "sk-placeholder"),
    )
    embedding = response.data[0].embedding
    print(f"向量维度: {len(embedding)}")
    print(f"前 5 维: {embedding[:5]}")


# ============================================================
# 示例 8：RetryPolicy 精细控制
# ============================================================
async def example_retry_policy():
    from litellm import Router
    from litellm.router import RetryPolicy

    router = Router(
        model_list=[
            {
                "model_name": "gpt-4o",
                "litellm_params": {
                    "model": "openai/gpt-4o",
                    "api_key": os.getenv("OPENAI_API_KEY", "sk-placeholder"),
                },
            },
        ],
        retry_policy=RetryPolicy(
            RateLimitErrorRetries=3,
            TimeoutErrorRetries=2,
            AuthenticationErrorRetries=0,
            BadRequestErrorRetries=1,
        ),
    )

    response = await router.acompletion(
        model="gpt-4o",
        messages=[{"role": "user", "content": "Hello"}],
    )
    print(response.choices[0].message.content)


# ============================================================
# 入口：选择运行哪个示例
# ============================================================
if __name__ == "__main__":
    print("=== 示例 2: Router 负载均衡 ===")
    asyncio.run(example_router_basic())

    print("\n=== 示例 3: 流式调用 ===")
    asyncio.run(example_stream())

    print("\n=== 示例 4: Fallback ===")
    asyncio.run(example_fallback())

    print("\n=== 示例 5: 自定义路由策略 ===")
    asyncio.run(example_custom_strategy())

    print("\n=== 示例 6: 错误处理 ===")
    asyncio.run(example_error_handling())

    print("\n=== 示例 7: Embedding ===")
    asyncio.run(example_embedding())

    print("\n=== 示例 8: RetryPolicy ===")
    asyncio.run(example_retry_policy())
