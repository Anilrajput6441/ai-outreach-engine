package ai

import (
	"context"
	"fmt"
	"log"
	"os"

	openai "github.com/sashabaranov/go-openai"
)

func GenerateEmail(prompt string) (string, error) {
	log.Println("recieved prompt", prompt)

	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("OPENROUTER_API_KEY environment variable is not set")
	}

	// 2. Configure for OpenRouter
	config := openai.DefaultConfig(apiKey)
	config.BaseURL = "https://openrouter.ai"

	client := openai.NewClientWithConfig(config)

	// 3. Make the Request
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			// Using a reliable free model
			Model: "google/gemini-2.0-flash-lite-preview-02-05:free",
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
		},
	)

	if err != nil {
		return "", fmt.Errorf("AI error: %v", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response choices returned from AI")
	}

	if err != nil {
		return "", err
	}

	return resp.Choices[0].Message.Content, nil
}
