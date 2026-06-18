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

// Config carries per-request provider settings from the browser extension.
// If Provider and APIKey are both empty the server falls back to ANTHROPIC_API_KEY env.
type Config struct {
	Provider string // "anthropic" | "openai" | "google"
	Model    string
	APIKey   string
}

func (c *Config) resolve() {
	if c.Provider == "" {
		c.Provider = "anthropic"
	}
	if c.APIKey == "" && c.Provider == "anthropic" {
		c.APIKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if c.Model == "" {
		switch c.Provider {
		case "openai":
			c.Model = "gpt-4o-mini"
		case "google":
			c.Model = "gemini-2.0-flash"
		default:
			c.Model = "claude-opus-4-8"
		}
	}
}

const systemPrompt = `You are a sharp, concise technical assistant. Answer directly — no preamble or filler. Use markdown: code blocks for code, bold for key terms, bullet lists for steps. Show working for math and logic. Write runnable code when asked.`

const visionPrompt = `You are a sharp exam assistant. Read every question visible on the screen and answer each one directly and accurately. Number your answers to match the question numbers. Be concise — no filler.`

// ── Text chat ────────────────────────────────────────────────────────────────

func AskLLM(messages []Message, cfg Config) (string, error) {
	cfg.resolve()
	if cfg.APIKey == "" {
		return "", fmt.Errorf("no API key configured")
	}
	switch cfg.Provider {
	case "openai":
		return askOpenAI(messages, cfg)
	case "google":
		return askGemini(messages, cfg)
	default:
		return askClaude(messages, cfg)
	}
}

// ── Vision / screenshot ──────────────────────────────────────────────────────

func AskVision(imageBase64 string, cfg Config) (string, error) {
	cfg.resolve()
	if cfg.APIKey == "" {
		return "", fmt.Errorf("no API key configured")
	}
	switch cfg.Provider {
	case "openai":
		return askOpenAIVision(imageBase64, cfg)
	case "google":
		return askGeminiVision(imageBase64, cfg)
	default:
		return askClaudeVision(imageBase64, cfg)
	}
}

// ── Anthropic ────────────────────────────────────────────────────────────────

type claudeRequest struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	System    string    `json:"system"`
	Messages  []Message `json:"messages"`
}

type claudeVisionRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	System    string          `json:"system"`
	Messages  []claudeVisionMsg `json:"messages"`
}

type claudeVisionMsg struct {
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

func askClaude(messages []Message, cfg Config) (string, error) {
	body, err := json.Marshal(claudeRequest{
		Model:     cfg.Model,
		MaxTokens: 1024,
		System:    systemPrompt,
		Messages:  messages,
	})
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("x-api-key", cfg.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")

	raw, err := doHTTP(req)
	if err != nil {
		return "", err
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
	return "", fmt.Errorf("no text in anthropic response")
}

func askClaudeVision(imageBase64 string, cfg Config) (string, error) {
	body, err := json.Marshal(claudeVisionRequest{
		Model:     cfg.Model,
		MaxTokens: 2048,
		System:    visionPrompt,
		Messages: []claudeVisionMsg{{
			Role: "user",
			Content: []contentBlock{
				{Type: "image", Source: &imageSource{Type: "base64", MediaType: "image/png", Data: imageBase64}},
				{Type: "text", Text: "Answer all questions visible on this screen."},
			},
		}},
	})
	if err != nil {
		return "", fmt.Errorf("marshal vision request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("x-api-key", cfg.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")

	raw, err := doHTTP(req)
	if err != nil {
		return "", err
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
	return "", fmt.Errorf("no text in anthropic response")
}

// ── OpenAI ───────────────────────────────────────────────────────────────────

type openAIRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	Messages  []openAIMessage `json:"messages"`
}

type openAIMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // string for text, []openAIContentPart for vision
}

type openAIContentPart struct {
	Type     string            `json:"type"`
	Text     string            `json:"text,omitempty"`
	ImageURL *openAIImageURL   `json:"image_url,omitempty"`
}

type openAIImageURL struct {
	URL string `json:"url"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

func askOpenAI(messages []Message, cfg Config) (string, error) {
	msgs := []openAIMessage{{Role: "system", Content: systemPrompt}}
	for _, m := range messages {
		msgs = append(msgs, openAIMessage{Role: m.Role, Content: m.Content})
	}

	body, err := json.Marshal(openAIRequest{Model: cfg.Model, MaxTokens: 1024, Messages: msgs})
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	req.Header.Set("content-type", "application/json")

	return parseOpenAI(req)
}

func askOpenAIVision(imageBase64 string, cfg Config) (string, error) {
	msgs := []openAIMessage{
		{Role: "system", Content: visionPrompt},
		{Role: "user", Content: []openAIContentPart{
			{Type: "image_url", ImageURL: &openAIImageURL{URL: "data:image/png;base64," + imageBase64}},
			{Type: "text", Text: "Answer all questions visible on this screen."},
		}},
	}

	body, err := json.Marshal(openAIRequest{Model: cfg.Model, MaxTokens: 2048, Messages: msgs})
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	req.Header.Set("content-type", "application/json")

	return parseOpenAI(req)
}

func parseOpenAI(req *http.Request) (string, error) {
	raw, err := doHTTP(req)
	if err != nil {
		return "", err
	}
	var or openAIResponse
	if err := json.Unmarshal(raw, &or); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if or.Error != nil {
		return "", fmt.Errorf("openai error %s: %s", or.Error.Type, or.Error.Message)
	}
	if len(or.Choices) == 0 {
		return "", fmt.Errorf("no choices in openai response")
	}
	return or.Choices[0].Message.Content, nil
}

// ── Google Gemini ─────────────────────────────────────────────────────────────

type geminiRequest struct {
	SystemInstruction *geminiContent   `json:"system_instruction,omitempty"`
	Contents          []geminiContent  `json:"contents"`
	GenerationConfig  geminiGenConfig  `json:"generationConfig"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text       string          `json:"text,omitempty"`
	InlineData *geminiInline   `json:"inline_data,omitempty"`
}

type geminiInline struct {
	MimeType string `json:"mime_type"`
	Data     string `json:"data"`
}

type geminiGenConfig struct {
	MaxOutputTokens int `json:"maxOutputTokens"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func askGemini(messages []Message, cfg Config) (string, error) {
	contents := make([]geminiContent, 0, len(messages))
	for _, m := range messages {
		role := m.Role
		if role == "assistant" {
			role = "model"
		}
		contents = append(contents, geminiContent{
			Role:  role,
			Parts: []geminiPart{{Text: m.Content}},
		})
	}

	body, err := json.Marshal(geminiRequest{
		SystemInstruction: &geminiContent{Parts: []geminiPart{{Text: systemPrompt}}},
		Contents:          contents,
		GenerationConfig:  geminiGenConfig{MaxOutputTokens: 1024},
	})
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", cfg.Model, cfg.APIKey)
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("content-type", "application/json")

	return parseGemini(req)
}

func askGeminiVision(imageBase64 string, cfg Config) (string, error) {
	body, err := json.Marshal(geminiRequest{
		SystemInstruction: &geminiContent{Parts: []geminiPart{{Text: visionPrompt}}},
		Contents: []geminiContent{{
			Role: "user",
			Parts: []geminiPart{
				{InlineData: &geminiInline{MimeType: "image/png", Data: imageBase64}},
				{Text: "Answer all questions visible on this screen."},
			},
		}},
		GenerationConfig: geminiGenConfig{MaxOutputTokens: 2048},
	})
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", cfg.Model, cfg.APIKey)
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("content-type", "application/json")

	return parseGemini(req)
}

func parseGemini(req *http.Request) (string, error) {
	raw, err := doHTTP(req)
	if err != nil {
		return "", err
	}
	var gr geminiResponse
	if err := json.Unmarshal(raw, &gr); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if gr.Error != nil {
		return "", fmt.Errorf("gemini error %d: %s", gr.Error.Code, gr.Error.Message)
	}
	if len(gr.Candidates) == 0 || len(gr.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no content in gemini response")
	}
	return gr.Candidates[0].Content.Parts[0].Text, nil
}

// ── Shared HTTP ───────────────────────────────────────────────────────────────

func doHTTP(req *http.Request) ([]byte, error) {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	return raw, nil
}
