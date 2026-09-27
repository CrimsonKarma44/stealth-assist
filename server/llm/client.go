package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Message is exported so main.go can decode directly into it.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Config carries per-request provider settings from the browser extension.
// If Provider and APIKey are both empty the server falls back to ANTHROPIC_API_KEY env.
type Config struct {
	Provider string // "anthropic" | "openai" | "google" | "xai" | "openrouter" | "nvidia"
	Model    string
	APIKey   string
}

// Retired Gemini model IDs still sent by older extension builds / saved settings.
var geminiModelAliases = map[string]string{
	"gemini-2.0-flash":      "gemini-3.6-flash",
	"gemini-2.0-flash-lite": "gemini-3.6-flash",
	"gemini-1.5-flash":      "gemini-3.6-flash",
	"gemini-1.5-flash-8b":   "gemini-3.1-flash-lite",
	"gemini-1.5-pro":        "gemini-2.5-pro",
}

func (c *Config) resolve() {
	if c.Provider == "" {
		c.Provider = "anthropic"
	}
	if c.APIKey == "" {
		switch c.Provider {
		case "anthropic":
			c.APIKey = os.Getenv("ANTHROPIC_API_KEY")
		case "xai":
			c.APIKey = os.Getenv("XAI_API_KEY")
		case "openrouter":
			c.APIKey = os.Getenv("OPENROUTER_API_KEY")
		case "nvidia":
			c.APIKey = os.Getenv("NVIDIA_API_KEY")
		}
	}
	if c.Model == "" {
		switch c.Provider {
		case "openai":
			c.Model = "gpt-4o-mini"
		case "google":
			c.Model = "gemini-3.6-flash"
		case "xai":
			c.Model = "grok-4.5"
		case "openrouter":
			c.Model = "openrouter/free"
		case "nvidia":
			c.Model = "moonshotai/kimi-k3"
		default:
			c.Model = "claude-opus-4-8"
		}
	}
	if c.Provider == "google" {
		if next, ok := geminiModelAliases[c.Model]; ok {
			c.Model = next
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
	case "xai":
		return askGrok(messages, cfg)
	case "openrouter":
		return askOpenRouter(messages, cfg)
	case "nvidia":
		return askNvidia(messages, cfg)
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
	case "xai":
		return askGrokVision(imageBase64, cfg)
	case "openrouter":
		return askOpenRouterVision(imageBase64, cfg)
	case "nvidia":
		return askNvidiaVision(imageBase64, cfg)
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
	Model     string            `json:"model"`
	MaxTokens int               `json:"max_tokens"`
	System    string            `json:"system"`
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
	Model           string          `json:"model"`
	MaxTokens       int             `json:"max_tokens"`
	Messages        []openAIMessage `json:"messages"`
	Temperature     *float64        `json:"temperature,omitempty"`
	TopP            *float64        `json:"top_p,omitempty"`
	ReasoningEffort any             `json:"reasoning_effort,omitempty"`
}

// openAICallOpts overrides sampling and token budget for one OpenAI-compatible call.
// Nil fields are omitted so other providers keep their previous request shape.
type openAICallOpts struct {
	maxTokens       int
	temperature     *float64
	topP            *float64
	reasoningEffort any
}

type openAIMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // string for text, []openAIContentPart for vision
}

type openAIContentPart struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	ImageURL *openAIImageURL `json:"image_url,omitempty"`
}

type openAIImageURL struct {
	URL string `json:"url"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

const (
	openAIChatURL     = "https://api.openai.com/v1/chat/completions"
	xaiChatURL        = "https://api.x.ai/v1/chat/completions"
	openRouterChatURL = "https://openrouter.ai/api/v1/chat/completions"
	nvidiaChatURL     = "https://integrate.api.nvidia.com/v1/chat/completions"
)

func askOpenAI(messages []Message, cfg Config) (string, error) {
	return askOpenAICompatible(messages, cfg, openAIChatURL, "openai", nil, nil)
}

func askOpenAIVision(imageBase64 string, cfg Config) (string, error) {
	return askOpenAICompatibleVision(imageBase64, cfg, openAIChatURL, "openai", nil, nil)
}

// xAI Grok uses an OpenAI-compatible chat completions API.
func askGrok(messages []Message, cfg Config) (string, error) {
	return askOpenAICompatible(messages, cfg, xaiChatURL, "xai", nil, nil)
}

func askGrokVision(imageBase64 string, cfg Config) (string, error) {
	return askOpenAICompatibleVision(imageBase64, cfg, xaiChatURL, "xai", nil, nil)
}

// OpenRouter is OpenAI-compatible and routes to many free/paid upstream models.
func askOpenRouter(messages []Message, cfg Config) (string, error) {
	return askOpenAICompatible(messages, cfg, openRouterChatURL, "openrouter", openRouterHeaders(), nil)
}

func askOpenRouterVision(imageBase64 string, cfg Config) (string, error) {
	return askOpenAICompatibleVision(imageBase64, cfg, openRouterChatURL, "openrouter", openRouterHeaders(), nil)
}

// NVIDIA NIM's hosted API is OpenAI-compatible. Every model offered in the
// extension accepts images, so chat and screenshot share one endpoint.
func askNvidia(messages []Message, cfg Config) (string, error) {
	return askOpenAICompatible(messages, cfg, nvidiaChatURL, "nvidia", nil, nvidiaCallOpts(cfg, false))
}

func askNvidiaVision(imageBase64 string, cfg Config) (string, error) {
	return askOpenAICompatibleVision(imageBase64, cfg, nvidiaChatURL, "nvidia", nil, nvidiaCallOpts(cfg, true))
}

func nvidiaCallOpts(cfg Config, vision bool) *openAICallOpts {
	maxTokens := 4096
	if vision {
		maxTokens = 8192
	}
	temp := 0.5
	topP := 1.0
	return &openAICallOpts{
		maxTokens:       maxTokens,
		temperature:     &temp,
		topP:            &topP,
		reasoningEffort: nvidiaReasoningEffort(cfg.Model),
	}
}

// Reasoning models spend the token budget before the answer. Keep effort low
// so a screenshot still returns content. Llama 3.2 Vision has no reasoning
// control. DeepSeek expects a number from 1 to 100; the others take low/medium/high.
func nvidiaReasoningEffort(model string) any {
	switch model {
	case "deepseek-ai/deepseek-v4.1-flash":
		return 20
	case "meta/llama-3.2-90b-vision-instruct", "meta/llama-3.2-11b-vision-instruct":
		return nil
	default:
		return "low"
	}
}

func openRouterHeaders() map[string]string {
	return map[string]string{
		"HTTP-Referer": "https://stealth-assist-1.onrender.com",
		"X-Title":      "Stealth Assist",
	}
}

func askOpenAICompatible(messages []Message, cfg Config, endpoint, label string, extraHeaders map[string]string, opts *openAICallOpts) (string, error) {
	msgs := []openAIMessage{{Role: "system", Content: systemPrompt}}
	for _, m := range messages {
		msgs = append(msgs, openAIMessage{Role: m.Role, Content: m.Content})
	}

	body, err := json.Marshal(openAIRequestFrom(cfg, 1024, msgs, opts))
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	req.Header.Set("content-type", "application/json")
	req.Header.Set("accept", "application/json")
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}

	return parseOpenAICompatible(req, label)
}

func askOpenAICompatibleVision(imageBase64 string, cfg Config, endpoint, label string, extraHeaders map[string]string, opts *openAICallOpts) (string, error) {
	msgs := []openAIMessage{
		{Role: "system", Content: visionPrompt},
		{Role: "user", Content: []openAIContentPart{
			{Type: "image_url", ImageURL: &openAIImageURL{URL: "data:image/png;base64," + imageBase64}},
			{Type: "text", Text: "Answer all questions visible on this screen."},
		}},
	}

	body, err := json.Marshal(openAIRequestFrom(cfg, 2048, msgs, opts))
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	req.Header.Set("content-type", "application/json")
	req.Header.Set("accept", "application/json")
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}

	return parseOpenAICompatible(req, label)
}

func openAIRequestFrom(cfg Config, fallbackTokens int, msgs []openAIMessage, opts *openAICallOpts) openAIRequest {
	req := openAIRequest{Model: cfg.Model, MaxTokens: fallbackTokens, Messages: msgs}
	if opts == nil {
		return req
	}
	if opts.maxTokens > 0 {
		req.MaxTokens = opts.maxTokens
	}
	req.Temperature = opts.temperature
	req.TopP = opts.topP
	req.ReasoningEffort = opts.reasoningEffort
	return req
}

func parseOpenAICompatible(req *http.Request, label string) (string, error) {
	res, err := doHTTPResult(req)
	if err != nil {
		return "", err
	}
	// A cold NVIDIA endpoint answers 202 with an empty body and NVCF-REQID.
	// The completion shows up on the status URL.
	if label == "nvidia" && res.Status == http.StatusAccepted {
		res, err = pollNvidia(req, res)
		if err != nil {
			return "", err
		}
	}
	return parseOpenAIBody(res, label)
}

func parseOpenAIBody(res httpResult, label string) (string, error) {
	var or openAIResponse
	if err := json.Unmarshal(res.Body, &or); err != nil {
		return "", fmt.Errorf("decode %s response (HTTP %d): %w; body: %s", label, res.Status, err, clip(res.Body))
	}
	if or.Error != nil {
		return "", fmt.Errorf("%s error %s: %s", label, or.Error.Type, or.Error.Message)
	}
	if len(or.Choices) == 0 {
		return "", providerProblem(res, label)
	}
	msg := or.Choices[0].Message
	if msg.Content == "" && msg.ReasoningContent != "" {
		return "", fmt.Errorf("%s returned reasoning but no answer (token budget used by reasoning)", label)
	}
	return msg.Content, nil
}

// providerProblem reads NVIDIA's problem-details body ({title, status, detail})
// and other non-completion JSON. That body has no choices array.
func providerProblem(res httpResult, label string) error {
	var problem struct {
		Title  string          `json:"title"`
		Status int             `json:"status"`
		Detail json.RawMessage `json:"detail"`
		Error  json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(res.Body, &problem); err != nil {
		return fmt.Errorf("%s returned HTTP %d: %s", label, res.Status, clip(res.Body))
	}
	detail := rawJSONText(problem.Detail)
	errText := rawJSONText(problem.Error)
	code := problem.Status
	if code == 0 {
		code = res.Status
	}
	switch {
	case detail != "" && problem.Title != "":
		return fmt.Errorf("%s: %s (%d %s)", label, detail, code, problem.Title)
	case detail != "":
		return fmt.Errorf("%s: %s (HTTP %d)", label, detail, code)
	case problem.Title != "":
		return fmt.Errorf("%s: %s (HTTP %d)", label, problem.Title, code)
	case errText != "":
		return fmt.Errorf("%s: %s (HTTP %d)", label, errText, code)
	default:
		return fmt.Errorf("%s returned HTTP %d with no choices: %s", label, res.Status, clip(res.Body))
	}
}

func pollNvidia(orig *http.Request, first httpResult) (httpResult, error) {
	reqID := first.Header.Get("NVCF-REQID")
	if reqID == "" {
		return httpResult{}, fmt.Errorf("nvidia returned 202 without NVCF-REQID")
	}
	deadline := time.Now().Add(45 * time.Second)
	res := first
	for res.Status == http.StatusAccepted {
		if time.Now().After(deadline) {
			return httpResult{}, fmt.Errorf("nvidia request %s still pending", reqID)
		}
		poll, err := http.NewRequest("GET", "https://integrate.api.nvidia.com/v1/status/"+reqID, nil)
		if err != nil {
			return httpResult{}, err
		}
		poll.Header.Set("Authorization", orig.Header.Get("Authorization"))
		poll.Header.Set("Accept", "application/json")
		res, err = doHTTPResult(poll)
		if err != nil {
			return httpResult{}, err
		}
		if res.Status == http.StatusAccepted {
			time.Sleep(time.Second)
		}
	}
	return res, nil
}

func rawJSONText(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(string(raw))
}

func clip(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 300 {
		return s[:300]
	}
	return s
}

// ── Google Gemini ─────────────────────────────────────────────────────────────

type geminiRequest struct {
	SystemInstruction *geminiContent  `json:"system_instruction,omitempty"`
	Contents          []geminiContent `json:"contents"`
	GenerationConfig  geminiGenConfig `json:"generationConfig"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text       string        `json:"text,omitempty"`
	InlineData *geminiInline `json:"inline_data,omitempty"`
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

// HTTPClient is the HTTP client used for all external API calls.
// Tests can override this to intercept requests.
var HTTPClient = http.DefaultClient

type httpResult struct {
	Status int
	Header http.Header
	Body   []byte
}

func doHTTP(req *http.Request) ([]byte, error) {
	res, err := doHTTPResult(req)
	if err != nil {
		return nil, err
	}
	return res.Body, nil
}

func doHTTPResult(req *http.Request) (httpResult, error) {
	resp, err := HTTPClient.Do(req)
	if err != nil {
		return httpResult{}, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return httpResult{}, fmt.Errorf("read response: %w", err)
	}
	return httpResult{Status: resp.StatusCode, Header: resp.Header, Body: raw}, nil
}
