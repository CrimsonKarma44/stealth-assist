package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// Message is exported so main.go can decode directly into it.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ── Text chat ────────────────────────────────────────────────────────────────

type claudeRequest struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	System    string    `json:"system"`
	Messages  []Message `json:"messages"`
}

type claudeResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

const systemPrompt = `You are a sharp, concise technical assistant. Answer directly — no preamble or filler. Use markdown: code blocks for code, bold for key terms, bullet lists for steps. Show working for math and logic. Write runnable code when asked.`

func AskLLM(messages []Message) (string, error) {
	return callClaude(claudeRequest{
		Model:     "claude-opus-4-8",
		MaxTokens: 1024,
		System:    systemPrompt,
		Messages:  messages,
	})
}

// ── Vision / screenshot ──────────────────────────────────────────────────────

type visionRequest struct {
	Model     string           `json:"model"`
	MaxTokens int              `json:"max_tokens"`
	System    string           `json:"system"`
	Messages  []visionMessage  `json:"messages"`
}

type visionMessage struct {
	Role    string         `json:"role"`
	Content []contentBlock `json:"content"`
}

type contentBlock struct {
	Type   string       `json:"type"`
	Source *imageSource `json:"source,omitempty"`
	Text   string       `json:"text,omitempty"`
}

type imageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

const visionPrompt = `You are a sharp exam assistant. Read every question visible on the screen and answer each one directly and accurately. Number your answers to match the question numbers. Be concise — no filler.`

func AskVision(imageBase64 string) (string, error) {
	req := visionRequest{
		Model:     "claude-opus-4-8",
		MaxTokens: 2048,
		System:    visionPrompt,
		Messages: []visionMessage{{
			Role: "user",
			Content: []contentBlock{
				{
					Type: "image",
					Source: &imageSource{
						Type:      "base64",
						MediaType: "image/png",
						Data:      imageBase64,
					},
				},
				{
					Type: "text",
					Text: "Answer all questions visible on this screen.",
				},
			},
		}},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal vision request: %w", err)
	}
	return doRequest(body)
}

// ── Shared HTTP logic ────────────────────────────────────────────────────────

func callClaude(req claudeRequest) (string, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}
	return doRequest(body)
}

func doRequest(body []byte) (string, error) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("ANTHROPIC_API_KEY not set")
	}

	req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	var cr claudeResponse
	if err := json.Unmarshal(raw, &cr); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	if cr.Error != nil {
		return "", fmt.Errorf("anthropic error %s: %s", cr.Error.Type, cr.Error.Message)
	}

	for _, block := range cr.Content {
		if block.Type == "text" {
			return block.Text, nil
		}
	}

	return "", fmt.Errorf("no text content in response")
}
