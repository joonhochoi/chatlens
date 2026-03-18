// Package llm provides a unified LLM client interface for Ollama, Gemini, and OpenAI.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"chatlens/internal/config"
)

const httpTimeout = 120 * time.Second

// Client is the common interface for all LLM providers.
type Client interface {
	Chat(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

// New returns the appropriate Client based on settings.LLMProvider.
func New(s config.Settings) (Client, error) {
	switch s.LLMProvider {
	case "gemini":
		if s.APIKey == "" {
			return nil, fmt.Errorf("Gemini API 키가 설정되지 않았습니다")
		}
		return &geminiClient{apiKey: s.APIKey}, nil
	case "openai":
		if s.APIKey == "" {
			return nil, fmt.Errorf("OpenAI API 키가 설정되지 않았습니다")
		}
		return &openaiClient{apiKey: s.APIKey}, nil
	default: // "ollama"
		model := s.OllamaModel
		if model == "" {
			model = "llama3:latest"
		}
		return &ollamaClient{model: model}, nil
	}
}

// ── Ollama ────────────────────────────────────────────────────────────────

type ollamaClient struct{ model string }

type ollamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaChatResponse struct {
	Message ollamaMessage `json:"message"`
}

func (c *ollamaClient) Chat(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	payload := ollamaChatRequest{
		Model: c.model,
		Messages: []ollamaMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Stream: false,
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequestWithContext(ctx, "POST", "http://localhost:11434/api/chat",
		bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{Timeout: httpTimeout}).Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama %d: %s", resp.StatusCode, string(b))
	}

	var result ollamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("ollama decode: %w", err)
	}
	return result.Message.Content, nil
}

// ── Gemini ────────────────────────────────────────────────────────────────

type geminiClient struct{ apiKey string }

type geminiRequest struct {
	SystemInstruction *geminiContent  `json:"system_instruction,omitempty"`
	Contents          []geminiContent `json:"contents"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
	Role  string       `json:"role,omitempty"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiResponse struct {
	Candidates []struct {
		Content geminiContent `json:"content"`
	} `json:"candidates"`
}

func (c *geminiClient) Chat(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	url := "https://generativelanguage.googleapis.com/v1beta/models/gemini-3.1-flash-lite-preview:generateContent?key=" + c.apiKey
	payload := geminiRequest{
		SystemInstruction: &geminiContent{
			Parts: []geminiPart{{Text: systemPrompt}},
		},
		Contents: []geminiContent{
			{Role: "user", Parts: []geminiPart{{Text: userPrompt}}},
		},
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{Timeout: httpTimeout}).Do(req)
	if err != nil {
		return "", fmt.Errorf("gemini request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("gemini %d: %s", resp.StatusCode, string(b))
	}

	var result geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("gemini decode: %w", err)
	}
	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("gemini: 빈 응답")
	}
	return result.Candidates[0].Content.Parts[0].Text, nil
}

// ── OpenAI ────────────────────────────────────────────────────────────────

type openaiClient struct{ apiKey string }

type openaiRequest struct {
	Model    string          `json:"model"`
	Messages []openaiMessage `json:"messages"`
}

type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openaiResponse struct {
	Choices []struct {
		Message openaiMessage `json:"message"`
	} `json:"choices"`
}

func (c *openaiClient) Chat(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	payload := openaiRequest{
		Model: "gpt-4o-mini",
		Messages: []openaiMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions",
		bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := (&http.Client{Timeout: httpTimeout}).Do(req)
	if err != nil {
		return "", fmt.Errorf("openai request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("openai %d: %s", resp.StatusCode, string(b))
	}

	var result openaiResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("openai decode: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("openai: 빈 응답")
	}
	return result.Choices[0].Message.Content, nil
}
