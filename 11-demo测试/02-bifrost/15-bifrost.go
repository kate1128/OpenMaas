/*
Bifrost Go SDK 真实使用示例

运行前提：
  go get github.com/maximhq/bifrost/core
*/

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	# ✅ 保持包的正确引入
	"github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/core/schemas"
)

// 为了配合测试，我们补充一个标准的 main 函数
func main() {
	fmt.Println("--- 开始测试示例 1：基础基础调用 ---")
	exampleBasicChat()

	fmt.Println("\n--- 开始测试示例 4：流式调用 ---")
	exampleStream()
}

// ============================================================
// 示例 1: 基础调用 — 单 Provider 单 Key
// ============================================================

type SingleProviderAccount struct{}

func (a *SingleProviderAccount) GetConfiguredProviders() ([]schemas.ModelProvider, error) {
	return []schemas.ModelProvider{schemas.OpenAI}, nil
}

func (a *SingleProviderAccount) GetKeysForProvider(ctx *context.Context, provider schemas.ModelProvider) ([]schemas.Key, error) {
	if provider == schemas.OpenAI {
		return []schemas.Key{{
			Value:  os.Getenv("OPENAI_API_KEY"),
			Models: schemas.WhiteList{"*"},
			Weight: 1.0,
		}}, nil
	}
	return nil, fmt.Errorf("unsupported provider: %s", provider)
}

func (a *SingleProviderAccount) GetConfigForProvider(provider schemas.ModelProvider) (*schemas.ProviderConfig, error) {
	if provider == schemas.OpenAI {
		return &schemas.ProviderConfig{
			NetworkConfig:            schemas.DefaultNetworkConfig,
			ConcurrencyAndBufferSize: schemas.DefaultConcurrencyAndBufferSize,
		}, nil
	}
	return nil, fmt.Errorf("unsupported provider: %s", provider)
}

func exampleBasicChat() {
	# ✅ 修正：将 bifrost.Init 改为 core.Init
	client, err := core.Init(context.Background(), schemas.BifrostConfig{
		Account: &SingleProviderAccount{},
	})
	if err != nil {
		log.Fatalf("Init failed: %v", err)
	}
	defer client.Shutdown()

	messages := []schemas.ChatMessage{
		{
			Role: schemas.ChatMessageRoleUser,
			Content: &schemas.ChatMessageContent{
				ContentStr: schemas.Ptr("用一句话解释量子计算"),
			},
		},
	}

	response, err := client.ChatCompletionRequest(
		schemas.NewBifrostContext(context.Background(), schemas.NoDeadline),
		&schemas.BifrostChatRequest{
			Provider: schemas.OpenAI,
			Model:    "gpt-4o-mini",
			Input:    messages,
			Params: &schemas.ChatParameters{
				Temperature: schemas.Ptr(0.7),
				MaxTokens:   schemas.Ptr(200),
			},
		},
	)
	if err != nil {
		log.Fatalf("Chat request failed: %v", err)
	}

	fmt.Printf("Response: %s\n", *response.Choices[0].Message.Content.ContentStr)
	fmt.Printf("Usage: %+v\n", response.Usage)
}

// ============================================================
// 示例 2: 多 Provider 配置 (OpenAI + Anthropic)
// ============================================================

type MultiProviderAccount struct{}

func (a *MultiProviderAccount) GetConfiguredProviders() ([]schemas.ModelProvider, error) {
	return []schemas.ModelProvider{schemas.OpenAI, schemas.Anthropic}, nil
}

func (a *MultiProviderAccount) GetKeysForProvider(ctx *context.Context, provider schemas.ModelProvider) ([]schemas.Key, error) {
	switch provider {
	case schemas.OpenAI:
		return []schemas.Key{{
			Value:  os.Getenv("OPENAI_API_KEY"),
			Models: schemas.WhiteList{"*"},
			Weight: 1.0,
		}}, nil
	case schemas.Anthropic:
		return []schemas.Key{{
			Value:  os.Getenv("ANTHROPIC_API_KEY"),
			Models: schemas.WhiteList{"*"},
			Weight: 1.0,
		}}, nil
	}
	return nil, fmt.Errorf("unsupported provider: %s", provider)
}

func (a *MultiProviderAccount) GetConfigForProvider(provider schemas.ModelProvider) (*schemas.ProviderConfig, error) {
	cfg := &schemas.ProviderConfig{
		NetworkConfig:            schemas.DefaultNetworkConfig,
		ConcurrencyAndBufferSize: schemas.DefaultConcurrencyAndBufferSize,
	}
	switch provider {
	case schemas.OpenAI:
		return cfg, nil
	case schemas.Anthropic:
		return cfg, nil
	}
	return nil, fmt.Errorf("unsupported provider: %s", provider)
}

func exampleMultiProvider() {
	# ✅ 修正：将 bifrost.Init 改为 core.Init
	client, err := core.Init(context.Background(), schemas.BifrostConfig{
		Account: &MultiProviderAccount{},
	})
	if err != nil {
		log.Fatalf("Init failed: %v", err)
	}
	defer client.Shutdown()

	messages := []schemas.ChatMessage{
		{
			Role: schemas.ChatMessageRoleUser,
			Content: &schemas.ChatMessageContent{
				ContentStr: schemas.Ptr("Hello from Bifrost Go SDK!"),
			},
		},
	}

	resp, err := client.ChatCompletionRequest(
		schemas.NewBifrostContext(context.Background(), schemas.NoDeadline),
		&schemas.BifrostChatRequest{
			Provider: schemas.OpenAI,
			Model:    "gpt-4o-mini",
			Input:    messages,
		},
	)
	if err == nil {
		fmt.Printf("OpenAI: %s\n", *resp.Choices[0].Message.Content.ContentStr)
	}

	resp, err = client.ChatCompletionRequest(
		schemas.NewBifrostContext(context.Background(), schemas.NoDeadline),
		&schemas.BifrostChatRequest{
			Provider: schemas.Anthropic,
			Model:    "claude-3-5-haiku-latest",
			Input:    messages,
		},
	)
	if err == nil {
		fmt.Printf("Anthropic: %s\n", *resp.Choices[0].Message.Content.ContentStr)
	}
}

// ============================================================
// 示例 3: 加权负载均衡 (两个 Key 70/30)
// ============================================================

type WeightedKeyAccount struct{}

func (a *WeightedKeyAccount) GetConfiguredProviders() ([]schemas.ModelProvider, error) {
	return []schemas.ModelProvider{schemas.OpenAI}, nil
}

func (a *WeightedKeyAccount) GetKeysForProvider(ctx *context.Context, provider schemas.ModelProvider) ([]schemas.Key, error) {
	if provider == schemas.OpenAI {
		return []schemas.Key{
			{
				Value:  os.Getenv("OPENAI_API_KEY"),
				Models: schemas.WhiteList{"*"},
				Weight: 0.7,
			},
			{
				Value:  os.Getenv("OPENAI_API_KEY_2"),
				Models: schemas.WhiteList{"*"},
				Weight: 0.3,
			},
		}, nil
	}
	return nil, fmt.Errorf("unsupported provider: %s", provider)
}

func (a *WeightedKeyAccount) GetConfigForProvider(provider schemas.ModelProvider) (*schemas.ProviderConfig, error) {
	if provider == schemas.OpenAI {
		return &schemas.ProviderConfig{
			NetworkConfig:            schemas.DefaultNetworkConfig,
			ConcurrencyAndBufferSize: schemas.DefaultConcurrencyAndBufferSize,
		}, nil
	}
	return nil, fmt.Errorf("unsupported provider: %s", provider)
}

func exampleWeightedKeys() {
	# ✅ 修正：将 bifrost.Init 改为 core.Init
	client, err := core.Init(context.Background(), schemas.BifrostConfig{
		Account: &WeightedKeyAccount{},
	})
	if err != nil {
		log.Fatalf("Init failed: %v", err)
	}
	defer client.Shutdown()

	for i := 0; i < 10; i++ {
		resp, err := client.ChatCompletionRequest(
			schemas.NewBifrostContext(context.Background(), schemas.NoDeadline),
			&schemas.BifrostChatRequest{
				Provider: schemas.OpenAI,
				Model:    "gpt-4o-mini",
				Input: []schemas.ChatMessage{
					{
						Role: schemas.ChatMessageRoleUser,
						Content: &schemas.ChatMessageContent{
							ContentStr: schemas.Ptr("ping"),
						},
					},
				},
			},
		)
		if err != nil {
			log.Printf("Request %d failed: %v", i, err)
			continue
		}
		fmt.Printf("Request %d: %s\n", i, *resp.Choices[0].Message.Content.ContentStr)
	}
}

// ============================================================
// 示例 4: 流式调用
// ============================================================

func exampleStream() {
	# ✅ 修正：将 bifrost.Init 改为 core.Init
	client, err := core.Init(context.Background(), schemas.BifrostConfig{
		Account: &SingleProviderAccount{},
	})
	if err != nil {
		log.Fatalf("Init failed: %v", err)
	}
	defer client.Shutdown()

	messages := []schemas.ChatMessage{
		{
			Role: schemas.ChatMessageRoleUser,
			Content: &schemas.ChatMessageContent{
				ContentStr: schemas.Ptr("从 1 数到 5"),
			},
		},
	}

	stream, err := client.ChatCompletionStreamRequest(
		schemas.NewBifrostContext(context.Background(), schemas.NoDeadline),
		&schemas.BifrostChatRequest{
			Provider: schemas.OpenAI,
			Model:    "gpt-4o-mini",
			Input:    messages,
		},
	)
	if err != nil {
		log.Fatalf("Stream request failed: %v", err)
	}

	for chunk := range stream {
		if chunk.BifrostError != nil {
			log.Printf("Stream error: %v", chunk.BifrostError)
			break
		}
		# ✅ 修正：增加多层 Nil 指针安全检查，防止 panic 闪退
		if chunk.BifrostChatResponse != nil && len(chunk.BifrostChatResponse.Choices) > 0 {
			choice := chunk.BifrostChatResponse.Choices[0]
			if choice.ChatStreamResponseChoice != nil &&
				choice.ChatStreamResponseChoice.Delta != nil &&
				choice.ChatStreamResponseChoice.Delta.Content != nil {
				fmt.Print(*choice.ChatStreamResponseChoice.Delta.Content)
			}
		}
	}
	fmt.Println()
}


// ============================================================
// 示例 5: 自定义重试策略
// ============================================================

type RetryConfigAccount struct{}

func (a *RetryConfigAccount) GetConfiguredProviders() ([]schemas.ModelProvider, error) {
	return []schemas.ModelProvider{schemas.OpenAI}, nil
}

func (a *RetryConfigAccount) GetKeysForProvider(ctx *context.Context, provider schemas.ModelProvider) ([]schemas.Key, error) {
	if provider == schemas.OpenAI {
		return []schemas.Key{{
			Value:  os.Getenv("OPENAI_API_KEY"),
			Models: schemas.WhiteList{"*"},
			Weight: 1.0,
		}}, nil
	}
	return nil, fmt.Errorf("unsupported provider: %s", provider)
}

func (a *RetryConfigAccount) GetConfigForProvider(provider schemas.ModelProvider) (*schemas.ProviderConfig, error) {
	if provider == schemas.OpenAI {
		return &schemas.ProviderConfig{
			NetworkConfig: schemas.NetworkConfig{
				MaxRetries:          5,
				RetryBackoffInitial: 500 * time.Millisecond,
				RetryBackoffMax:     10 * time.Second,
			},
			ConcurrencyAndBufferSize: schemas.DefaultConcurrencyAndBufferSize,
		}, nil
	}
	return nil, fmt.Errorf("unsupported provider: %s", provider)
}

func exampleRetryConfig() {
	client, err := bifrost.Init(context.Background(), schemas.BifrostConfig{
		Account: &RetryConfigAccount{},
	})
	if err != nil {
		log.Fatalf("Init failed: %v", err)
	}
	defer client.Shutdown()

	resp, err := client.ChatCompletionRequest(
		schemas.NewBifrostContext(context.Background(), schemas.NoDeadline),
		&schemas.BifrostChatRequest{
			Provider: schemas.OpenAI,
			Model:    "gpt-4o-mini",
			Input: []schemas.ChatMessage{
				{
					Role: schemas.ChatMessageRoleUser,
					Content: &schemas.ChatMessageContent{
						ContentStr: schemas.Ptr("Hello"),
					},
				},
			},
		},
	)
	if err != nil {
		log.Fatalf("Request failed after retries: %v", err)
	}
	fmt.Printf("Response: %s\n", *resp.Choices[0].Message.Content.ContentStr)
}

// ============================================================
// 示例 6: Tool Calling (函数调用)
// ============================================================

func exampleToolCalling() {
	client, err := bifrost.Init(context.Background(), schemas.BifrostConfig{
		Account: &SingleProviderAccount{},
	})
	if err != nil {
		log.Fatalf("Init failed: %v", err)
	}
	defer client.Shutdown()

	calculatorTool := schemas.ChatTool{
		Type: schemas.ChatToolTypeFunction,
		Function: &schemas.ChatToolFunction{
			Name:        "calculator",
			Description: schemas.Ptr("一个计算器工具"),
			Parameters: &schemas.ToolFunctionParameters{
				Type: "object",
				Properties: map[string]interface{}{
					"operation": map[string]interface{}{
						"type": "string",
						"enum": []string{"add", "subtract", "multiply", "divide"},
					},
					"a": map[string]interface{}{"type": "number"},
					"b": map[string]interface{}{"type": "number"},
				},
				Required: []string{"operation", "a", "b"},
			},
		},
	}

	resp, err := client.ChatCompletionRequest(
		schemas.NewBifrostContext(context.Background(), schemas.NoDeadline),
		&schemas.BifrostChatRequest{
			Provider: schemas.OpenAI,
			Model:    "gpt-4o-mini",
			Input: []schemas.ChatMessage{
				{
					Role: schemas.ChatMessageRoleUser,
					Content: &schemas.ChatMessageContent{
						ContentStr: schemas.Ptr("2+2 等于多少？用计算器"),
					},
				},
			},
			Params: &schemas.ChatParameters{
				Tools: []schemas.ChatTool{calculatorTool},
			},
		},
	)
	if err != nil {
		log.Fatalf("Tool calling failed: %v", err)
	}

	msg := resp.Choices[0].Message
	if msg.ChatAssistantMessage != nil && msg.ChatAssistantMessage.ToolCalls != nil {
		for _, tc := range msg.ChatAssistantMessage.ToolCalls {
			fmt.Printf("Tool call: %s(%s)\n", *tc.Function.Name, tc.Function.Arguments)
		}
	} else {
		fmt.Printf("Response: %s\n", *msg.Content.ContentStr)
	}
}

// ============================================================
// 示例 7: 自定义 Base URL (对接本地 vLLM / Ollama)
// ============================================================

type CustomBaseURLAccount struct{}

func (a *CustomBaseURLAccount) GetConfiguredProviders() ([]schemas.ModelProvider, error) {
	return []schemas.ModelProvider{schemas.OpenAI}, nil
}

func (a *CustomBaseURLAccount) GetKeysForProvider(ctx *context.Context, provider schemas.ModelProvider) ([]schemas.Key, error) {
	if provider == schemas.OpenAI {
		return []schemas.Key{{
			Value:  "sk-local", // 本地服务一般不校验 Key
			Models: schemas.WhiteList{"*"},
			Weight: 1.0,
		}}, nil
	}
	return nil, fmt.Errorf("unsupported provider: %s", provider)
}

func (a *CustomBaseURLAccount) GetConfigForProvider(provider schemas.ModelProvider) (*schemas.ProviderConfig, error) {
	if provider == schemas.OpenAI {
		return &schemas.ProviderConfig{
			NetworkConfig: schemas.NetworkConfig{
				BaseURL: "http://localhost:8000/v1", // vLLM / Ollama 本地地址
			},
			ConcurrencyAndBufferSize: schemas.DefaultConcurrencyAndBufferSize,
		}, nil
	}
	return nil, fmt.Errorf("unsupported provider: %s", provider)
}

func exampleCustomBaseURL() {
	client, err := bifrost.Init(context.Background(), schemas.BifrostConfig{
		Account: &CustomBaseURLAccount{},
	})
	if err != nil {
		log.Fatalf("Init failed: %v", err)
	}
	defer client.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := client.ChatCompletionRequest(
		schemas.NewBifrostContext(ctx, schemas.NoDeadline),
		&schemas.BifrostChatRequest{
			Provider: schemas.OpenAI,
			Model:    "Qwen/Qwen2.5-7B-Instruct",
			Input: []schemas.ChatMessage{
				{
					Role: schemas.ChatMessageRoleUser,
					Content: &schemas.ChatMessageContent{
						ContentStr: schemas.Ptr("Hello"),
					},
				},
			},
		},
	)
	if err != nil {
		log.Printf("Local model request failed: %v", err)
		return
	}
	fmt.Printf("Local model: %s\n", *resp.Choices[0].Message.Content.ContentStr)
}

// ============================================================
// 示例 8: 错误处理
// ============================================================

func exampleErrorHandling() {
	// 用错误的 Key 模拟认证失败
	client, err := bifrost.Init(context.Background(), schemas.BifrostConfig{
		Account: &BadKeyAccount{},
	})
	if err != nil {
		log.Fatalf("Init failed: %v", err)
	}
	defer client.Shutdown()

	_, err = client.ChatCompletionRequest(
		schemas.NewBifrostContext(context.Background(), schemas.NoDeadline),
		&schemas.BifrostChatRequest{
			Provider: schemas.OpenAI,
			Model:    "gpt-4o-mini",
			Input: []schemas.ChatMessage{
				{
					Role: schemas.ChatMessageRoleUser,
					Content: &schemas.ChatMessageContent{
						ContentStr: schemas.Ptr("Hello"),
					},
				},
			},
		},
	)
	if err != nil {
		// BifrostError 包含标准错误信息
		if bfErr, ok := err.(*schemas.BifrostError); ok {
			fmt.Printf("Bifrost Error: code=%s, message=%s\n", bfErr.Error.Code, bfErr.Error.Message)
		} else {
			fmt.Printf("General error: %v\n", err)
		}
	}
}

type BadKeyAccount struct{}

func (a *BadKeyAccount) GetConfiguredProviders() ([]schemas.ModelProvider, error) {
	return []schemas.ModelProvider{schemas.OpenAI}, nil
}

func (a *BadKeyAccount) GetKeysForProvider(ctx *context.Context, provider schemas.ModelProvider) ([]schemas.Key, error) {
	if provider == schemas.OpenAI {
		return []schemas.Key{{
			Value:  "sk-bad-key",
			Models: schemas.WhiteList{"*"},
			Weight: 1.0,
		}}, nil
	}
	return nil, fmt.Errorf("unsupported provider: %s", provider)
}

func (a *BadKeyAccount) GetConfigForProvider(provider schemas.ModelProvider) (*schemas.ProviderConfig, error) {
	if provider == schemas.OpenAI {
		return &schemas.ProviderConfig{
			NetworkConfig:            schemas.DefaultNetworkConfig,
			ConcurrencyAndBufferSize: schemas.DefaultConcurrencyAndBufferSize,
		}, nil
	}
	return nil, fmt.Errorf("unsupported provider: %s", provider)
}

// ============================================================
// 示例 9: Embedding
// ============================================================

func exampleEmbedding() {
	client, err := bifrost.Init(context.Background(), schemas.BifrostConfig{
		Account: &SingleProviderAccount{},
	})
	if err != nil {
		log.Fatalf("Init failed: %v", err)
	}
	defer client.Shutdown()

	resp, err := client.EmbeddingRequest(
		schemas.NewBifrostContext(context.Background(), schemas.NoDeadline),
		&schemas.BifrostEmbeddingRequest{
			Provider: schemas.OpenAI,
			Model:    "text-embedding-3-small",
			Input:    schemas.Ptr("Bifrost AI Gateway 是什么"),
		},
	)
	if err != nil {
		log.Fatalf("Embedding failed: %v", err)
	}
	if len(resp.Data) > 0 {
		fmt.Printf("Embedding 维度: %d\n", len(resp.Data[0].Embedding))
		fmt.Printf("前 5 维: %v\n", resp.Data[0].Embedding[:5])
	}
}

// ============================================================
// 入口
// ============================================================

func main() {
	fmt.Println("=== 示例 1: 基础 Chat ===")
	exampleBasicChat()

	fmt.Println("\n=== 示例 2: 多 Provider ===")
	exampleMultiProvider()

	fmt.Println("\n=== 示例 3: 加权负载均衡 ===")
	exampleWeightedKeys()

	fmt.Println("\n=== 示例 4: 流式调用 ===")
	exampleStream()

	fmt.Println("\n=== 示例 5: 自定义重试 ===")
	exampleRetryConfig()

	fmt.Println("\n=== 示例 6: Tool Calling ===")
	exampleToolCalling()

	fmt.Println("\n=== 示例 7: 自定义 Base URL ===")
	exampleCustomBaseURL()

	fmt.Println("\n=== 示例 8: 错误处理 ===")
	exampleErrorHandling()

	fmt.Println("\n=== 示例 9: Embedding ===")
	exampleEmbedding()
}
