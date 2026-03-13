// Package main demonstrates the simplest way to use LLM Gateway as a Go SDK.
//
// No config files needed — just set your API key and run:
//
//	export DASHSCOPE_API_KEY=sk-xxx
//	go run examples/basic/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/lex1ng/llm-gateway/pkg/gateway"
	"github.com/lex1ng/llm-gateway/pkg/types"
)

func main() {
	apiKey := os.Getenv("DASHSCOPE_API_KEY")
	if apiKey == "" {
		log.Fatal("DASHSCOPE_API_KEY is required")
	}

	// 1. Create client — zero config files
	client, err := gateway.NewBuilder().
		AddProvider("alibaba", gateway.ProviderOpts{
			BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1",
			APIKey:  apiKey,
		}).
		Build()
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	// 2. Non-streaming chat
	fmt.Println("=== Chat ===")
	resp, err := client.Chat(context.Background(), &types.ChatRequest{
		Provider: "alibaba",
		Model:    "qwen-plus",
		Messages: []types.Message{
			{Role: types.RoleUser, Content: types.NewTextContent("用一句话介绍Go语言")},
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Model: %s\n", resp.Model)
	fmt.Printf("Reply: %s\n", resp.Content)
	fmt.Printf("Tokens: prompt=%d, completion=%d\n\n",
		resp.Usage.PromptTokens, resp.Usage.CompletionTokens)

	// 3. Streaming chat
	fmt.Println("=== Stream ===")
	stream, err := client.ChatStream(context.Background(), &types.ChatRequest{
		Provider: "alibaba",
		Model:    "qwen-turbo",
		Stream:   true,
		Messages: []types.Message{
			{Role: types.RoleUser, Content: types.NewTextContent("写一首关于代码的两行诗")},
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	for event := range stream {
		switch event.Type {
		case types.StreamEventContentDelta:
			fmt.Print(event.Delta)
		case types.StreamEventDone:
			fmt.Println()
		case types.StreamEventError:
			log.Printf("Stream error: %s", event.Error)
		}
	}

	// 4. Embedding
	fmt.Println("\n=== Embedding ===")
	embedResp, err := client.Embed(context.Background(), &types.EmbedRequest{
		Provider: "alibaba",
		Model:    "text-embedding-v3",
		Input:    []string{"你好世界", "Hello world"},
	})
	if err != nil {
		log.Fatal(err)
	}
	for i, item := range embedResp.Data {
		fmt.Printf("embedding[%d]: %d dims\n", i, len(item.Embedding))
	}
	fmt.Printf("Tokens: %d\n", embedResp.Usage.TotalTokens)
}
