package ai

import (
	"context"
	"log"

	"google.golang.org/genai"
)

func GenerateEmail(prompt string) (string, error) {
	log.Println("recieved prompt", prompt)
	ctx := context.Background()
	// The client gets the API key from the environment variable `GEMINI_API_KEY`.
	client, err := genai.NewClient(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}

	// ---- System prompt that tells the model what to do ----
	systemPrompt := `You are a professional email copywriter. 
Write a concise cold email to HR. 
Follow this exact structure:
Subject: <subject line>
Greeting: <e.g., Dear Robert,>
Body: <2 or 3 short sentences that introduce you and your value proposition>
Closing: <e.g., Best regards, Your Name> 
Do NOT include any extra text or explanations.`

	// Combine system + user prompt
	fullPrompt := systemPrompt + "\n\n" + prompt

	result, err := client.Models.GenerateContent(
		ctx,
		"gemini-2.5-flash",
		genai.Text(fullPrompt),
		nil,
	)
	if err != nil {
		log.Fatal(err)
	}
	return result.Text(), nil

}
