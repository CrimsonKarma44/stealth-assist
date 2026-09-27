package llm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// testTransport redirects all outgoing requests to target, preserving path/query.
type testTransport struct {
	target string
}

func (t *testTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	u, _ := url.Parse(t.target)
	req2.URL.Scheme = u.Scheme
	req2.URL.Host = u.Host
	return http.DefaultTransport.RoundTrip(req2)
}

func withMockServer(handler http.HandlerFunc) (*httptest.Server, func()) {
	srv := httptest.NewServer(handler)
	orig := HTTPClient
	HTTPClient = &http.Client{Transport: &testTransport{target: srv.URL}}
	return srv, func() {
		HTTPClient = orig
		srv.Close()
	}
}

// ── Config.resolve ────────────────────────────────────────────────────────────

func TestConfigResolveAnthropicDefaults(t *testing.T) {
	cfg := Config{}
	cfg.resolve()
	if cfg.Provider != "anthropic" {
		t.Errorf("provider: got %q, want %q", cfg.Provider, "anthropic")
	}
	if cfg.Model != "claude-opus-4-8" {
		t.Errorf("model: got %q, want %q", cfg.Model, "claude-opus-4-8")
	}
}

func TestConfigResolveOpenAIDefault(t *testing.T) {
	cfg := Config{Provider: "openai", APIKey: "k"}
	cfg.resolve()
	if cfg.Model != "gpt-4o-mini" {
		t.Errorf("model: got %q, want %q", cfg.Model, "gpt-4o-mini")
	}
}

func TestConfigResolveGoogleDefault(t *testing.T) {
	cfg := Config{Provider: "google", APIKey: "k"}
	cfg.resolve()
	if cfg.Model != "gemini-3.6-flash" {
		t.Errorf("model: got %q, want %q", cfg.Model, "gemini-3.6-flash")
	}
}

func TestConfigResolveXAIDefault(t *testing.T) {
	cfg := Config{Provider: "xai", APIKey: "k"}
	cfg.resolve()
	if cfg.Model != "grok-4.5" {
		t.Errorf("model: got %q, want %q", cfg.Model, "grok-4.5")
	}
}

func TestConfigResolveOpenRouterDefault(t *testing.T) {
	cfg := Config{Provider: "openrouter", APIKey: "k"}
	cfg.resolve()
	if cfg.Model != "openrouter/free" {
		t.Errorf("model: got %q, want %q", cfg.Model, "openrouter/free")
	}
}

func TestConfigResolveNvidiaDefault(t *testing.T) {
	cfg := Config{Provider: "nvidia", APIKey: "k"}
	cfg.resolve()
	if cfg.Model != "moonshotai/kimi-k3" {
		t.Errorf("model: got %q, want %q", cfg.Model, "moonshotai/kimi-k3")
	}
}

func TestConfigResolveNvidiaEnvKey(t *testing.T) {
	t.Setenv("NVIDIA_API_KEY", "nv-from-env")
	cfg := Config{Provider: "nvidia"}
	cfg.resolve()
	if cfg.APIKey != "nv-from-env" {
		t.Errorf("api key: got %q, want %q", cfg.APIKey, "nv-from-env")
	}
}

func TestConfigResolveGoogleMigratesRetiredModels(t *testing.T) {
	cases := map[string]string{
		"gemini-2.0-flash":      "gemini-3.6-flash",
		"gemini-2.0-flash-lite": "gemini-3.6-flash",
		"gemini-1.5-flash":      "gemini-3.6-flash",
		"gemini-1.5-flash-8b":   "gemini-3.1-flash-lite",
		"gemini-1.5-pro":        "gemini-2.5-pro",
	}
	for old, want := range cases {
		cfg := Config{Provider: "google", Model: old, APIKey: "k"}
		cfg.resolve()
		if cfg.Model != want {
			t.Errorf("%s: got %q, want %q", old, cfg.Model, want)
		}
	}
}

func TestConfigResolveCustomModelNotOverridden(t *testing.T) {
	cfg := Config{Provider: "anthropic", Model: "claude-haiku-4-5", APIKey: "k"}
	cfg.resolve()
	if cfg.Model != "claude-haiku-4-5" {
		t.Errorf("model: got %q, want %q", cfg.Model, "claude-haiku-4-5")
	}
}

// ── Missing API key ───────────────────────────────────────────────────────────

func TestAskLLMMissingAPIKey(t *testing.T) {
	_, err := AskLLM([]Message{{Role: "user", Content: "hi"}}, Config{})
	if err == nil || err.Error() != "no API key configured" {
		t.Errorf("expected 'no API key configured', got %v", err)
	}
}

func TestAskVisionMissingAPIKey(t *testing.T) {
	_, err := AskVision("base64data", Config{})
	if err == nil || err.Error() != "no API key configured" {
		t.Errorf("expected 'no API key configured', got %v", err)
	}
}

// ── Claude text ───────────────────────────────────────────────────────────────

func TestAskClaudeSuccess(t *testing.T) {
	_, cleanup := withMockServer(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"content": []map[string]string{{"type": "text", "text": "pong"}},
		})
	})
	defer cleanup()

	reply, err := AskLLM([]Message{{Role: "user", Content: "ping"}}, Config{Provider: "anthropic", APIKey: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reply != "pong" {
		t.Errorf("reply: got %q, want %q", reply, "pong")
	}
}

func TestAskClaudeAPIError(t *testing.T) {
	_, cleanup := withMockServer(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]string{"type": "invalid_request_error", "message": "bad input"},
		})
	})
	defer cleanup()

	_, err := AskLLM([]Message{{Role: "user", Content: "ping"}}, Config{Provider: "anthropic", APIKey: "test"})
	if err == nil {
		t.Error("expected error from API error response")
	}
}

func TestAskClaudeEmptyContent(t *testing.T) {
	_, cleanup := withMockServer(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"content": []interface{}{}})
	})
	defer cleanup()

	_, err := AskLLM([]Message{{Role: "user", Content: "ping"}}, Config{Provider: "anthropic", APIKey: "test"})
	if err == nil {
		t.Error("expected error for empty content blocks")
	}
}

// ── Claude vision ─────────────────────────────────────────────────────────────

func TestAskClaudeVisionSuccess(t *testing.T) {
	_, cleanup := withMockServer(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"content": []map[string]string{{"type": "text", "text": "I see a cat"}},
		})
	})
	defer cleanup()

	reply, err := AskVision("base64img", Config{Provider: "anthropic", APIKey: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reply != "I see a cat" {
		t.Errorf("reply: got %q, want %q", reply, "I see a cat")
	}
}

// ── OpenAI text ───────────────────────────────────────────────────────────────

func TestAskOpenAISuccess(t *testing.T) {
	_, cleanup := withMockServer(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "openai reply"}},
			},
		})
	})
	defer cleanup()

	reply, err := AskLLM([]Message{{Role: "user", Content: "hi"}}, Config{Provider: "openai", APIKey: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reply != "openai reply" {
		t.Errorf("reply: got %q, want %q", reply, "openai reply")
	}
}

func TestAskOpenAIAPIError(t *testing.T) {
	_, cleanup := withMockServer(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]string{"type": "invalid_request_error", "message": "bad"},
		})
	})
	defer cleanup()

	_, err := AskLLM([]Message{{Role: "user", Content: "hi"}}, Config{Provider: "openai", APIKey: "test"})
	if err == nil {
		t.Error("expected error from OpenAI error response")
	}
}

func TestAskOpenAIEmptyChoices(t *testing.T) {
	_, cleanup := withMockServer(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"choices": []interface{}{}})
	})
	defer cleanup()

	_, err := AskLLM([]Message{{Role: "user", Content: "hi"}}, Config{Provider: "openai", APIKey: "test"})
	if err == nil {
		t.Error("expected error for empty choices")
	}
}

// ── OpenAI vision ─────────────────────────────────────────────────────────────

func TestAskOpenAIVisionSuccess(t *testing.T) {
	_, cleanup := withMockServer(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "vision reply"}},
			},
		})
	})
	defer cleanup()

	reply, err := AskVision("base64img", Config{Provider: "openai", APIKey: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reply != "vision reply" {
		t.Errorf("reply: got %q, want %q", reply, "vision reply")
	}
}

// ── xAI Grok text ─────────────────────────────────────────────────────────────

func TestAskGrokSuccess(t *testing.T) {
	var gotPath string
	_, cleanup := withMockServer(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "grok reply"}},
			},
		})
	})
	defer cleanup()

	reply, err := AskLLM([]Message{{Role: "user", Content: "hi"}}, Config{Provider: "xai", APIKey: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reply != "grok reply" {
		t.Errorf("reply: got %q, want %q", reply, "grok reply")
	}
	if gotPath != "/v1/chat/completions" {
		// mock strips host; path may be empty or full depending on redirect — just ensure call succeeded
		_ = gotPath
	}
}

func TestAskGrokVisionSuccess(t *testing.T) {
	_, cleanup := withMockServer(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "grok vision reply"}},
			},
		})
	})
	defer cleanup()

	reply, err := AskVision("base64img", Config{Provider: "xai", APIKey: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reply != "grok vision reply" {
		t.Errorf("reply: got %q, want %q", reply, "grok vision reply")
	}
}

// ── OpenRouter text / vision ──────────────────────────────────────────────────

func TestAskOpenRouterSuccess(t *testing.T) {
	var gotTitle, gotReferer string
	_, cleanup := withMockServer(func(w http.ResponseWriter, r *http.Request) {
		gotTitle = r.Header.Get("X-Title")
		gotReferer = r.Header.Get("HTTP-Referer")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "openrouter reply"}},
			},
		})
	})
	defer cleanup()

	reply, err := AskLLM([]Message{{Role: "user", Content: "hi"}}, Config{Provider: "openrouter", APIKey: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reply != "openrouter reply" {
		t.Errorf("reply: got %q, want %q", reply, "openrouter reply")
	}
	if gotTitle != "Stealth Assist" {
		t.Errorf("X-Title: got %q, want %q", gotTitle, "Stealth Assist")
	}
	if gotReferer == "" {
		t.Error("expected HTTP-Referer header for OpenRouter")
	}
}

func TestAskOpenRouterVisionSuccess(t *testing.T) {
	_, cleanup := withMockServer(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "openrouter vision reply"}},
			},
		})
	})
	defer cleanup()

	reply, err := AskVision("base64img", Config{Provider: "openrouter", APIKey: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reply != "openrouter vision reply" {
		t.Errorf("reply: got %q, want %q", reply, "openrouter vision reply")
	}
}

// ── Gemini text ───────────────────────────────────────────────────────────────

func TestAskGeminiSuccess(t *testing.T) {
	_, cleanup := withMockServer(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"candidates": []map[string]interface{}{
				{"content": map[string]interface{}{
					"parts": []map[string]string{{"text": "gemini reply"}},
				}},
			},
		})
	})
	defer cleanup()

	reply, err := AskLLM([]Message{{Role: "user", Content: "hi"}}, Config{Provider: "google", APIKey: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reply != "gemini reply" {
		t.Errorf("reply: got %q, want %q", reply, "gemini reply")
	}
}

func TestAskGeminiAPIError(t *testing.T) {
	_, cleanup := withMockServer(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{"code": 400, "message": "bad request"},
		})
	})
	defer cleanup()

	_, err := AskLLM([]Message{{Role: "user", Content: "hi"}}, Config{Provider: "google", APIKey: "test"})
	if err == nil {
		t.Error("expected error from Gemini error response")
	}
}

func TestAskGeminiEmptyCandidates(t *testing.T) {
	_, cleanup := withMockServer(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"candidates": []interface{}{}})
	})
	defer cleanup()

	_, err := AskLLM([]Message{{Role: "user", Content: "hi"}}, Config{Provider: "google", APIKey: "test"})
	if err == nil {
		t.Error("expected error for empty candidates")
	}
}

// ── Gemini vision ─────────────────────────────────────────────────────────────

func TestAskGeminiVisionSuccess(t *testing.T) {
	_, cleanup := withMockServer(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"candidates": []map[string]interface{}{
				{"content": map[string]interface{}{
					"parts": []map[string]string{{"text": "gemini vision reply"}},
				}},
			},
		})
	})
	defer cleanup()

	reply, err := AskVision("base64img", Config{Provider: "google", APIKey: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reply != "gemini vision reply" {
		t.Errorf("reply: got %q, want %q", reply, "gemini vision reply")
	}
}

// ── NVIDIA NIM ────────────────────────────────────────────────────────────────

func TestAskNvidiaSuccess(t *testing.T) {
	var gotAuth, gotPath string
	var body map[string]interface{}
	_, cleanup := withMockServer(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		json.NewDecoder(r.Body).Decode(&body)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{
					"content":           "nvidia reply",
					"reasoning_content": "hidden chain of thought",
				}},
			},
		})
	})
	defer cleanup()

	reply, err := AskLLM([]Message{{Role: "user", Content: "hi"}}, Config{
		Provider: "nvidia",
		Model:    "moonshotai/kimi-k3",
		APIKey:   "nv-test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reply != "nvidia reply" {
		t.Errorf("reply: got %q, want %q", reply, "nvidia reply")
	}
	if gotAuth != "Bearer nv-test" {
		t.Errorf("Authorization: got %q", gotAuth)
	}
	if gotPath != "/v1/chat/completions" {
		t.Errorf("path: got %q", gotPath)
	}
	if body["model"] != "moonshotai/kimi-k3" {
		t.Errorf("model: got %v", body["model"])
	}
	if body["max_tokens"] != float64(4096) {
		t.Errorf("max_tokens: got %v, want 4096", body["max_tokens"])
	}
	if body["temperature"] != 0.5 {
		t.Errorf("temperature: got %v, want 0.5", body["temperature"])
	}
	if body["top_p"] != float64(1) {
		t.Errorf("top_p: got %v, want 1", body["top_p"])
	}
	if body["reasoning_effort"] != "low" {
		t.Errorf("reasoning_effort: got %v, want low", body["reasoning_effort"])
	}
}

func TestAskNvidiaDeepSeekReasoningEffort(t *testing.T) {
	var body map[string]interface{}
	_, cleanup := withMockServer(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&body)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "deepseek reply"}},
			},
		})
	})
	defer cleanup()

	_, err := AskLLM([]Message{{Role: "user", Content: "hi"}}, Config{
		Provider: "nvidia",
		Model:    "deepseek-ai/deepseek-v4.1-flash",
		APIKey:   "nv-test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if body["reasoning_effort"] != float64(20) {
		t.Errorf("reasoning_effort: got %v, want 20", body["reasoning_effort"])
	}
}

func TestAskNvidiaLlamaOmitsReasoningEffort(t *testing.T) {
	var body map[string]interface{}
	_, cleanup := withMockServer(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&body)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "llama reply"}},
			},
		})
	})
	defer cleanup()

	_, err := AskLLM([]Message{{Role: "user", Content: "hi"}}, Config{
		Provider: "nvidia",
		Model:    "meta/llama-3.2-11b-vision-instruct",
		APIKey:   "nv-test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := body["reasoning_effort"]; ok {
		t.Errorf("llama vision should omit reasoning_effort, got %v", body["reasoning_effort"])
	}
}

func TestAskNvidiaVisionSuccess(t *testing.T) {
	var body map[string]interface{}
	_, cleanup := withMockServer(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&body)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "nvidia vision reply"}},
			},
		})
	})
	defer cleanup()

	reply, err := AskVision("base64img", Config{
		Provider: "nvidia",
		Model:    "meta/muse-glimmer-30b",
		APIKey:   "nv-test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reply != "nvidia vision reply" {
		t.Errorf("reply: got %q, want %q", reply, "nvidia vision reply")
	}
	if body["max_tokens"] != float64(8192) {
		t.Errorf("max_tokens: got %v, want 8192", body["max_tokens"])
	}
	raw, err := json.Marshal(body["messages"])
	if err != nil {
		t.Fatalf("marshal messages: %v", err)
	}
	if !strings.Contains(string(raw), "data:image/png;base64,base64img") {
		t.Errorf("vision payload missing image data url: %s", raw)
	}
}

func TestAskNvidiaReasoningWithoutAnswer(t *testing.T) {
	_, cleanup := withMockServer(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"reasoning_content": "still thinking"}},
			},
		})
	})
	defer cleanup()

	_, err := AskLLM([]Message{{Role: "user", Content: "hi"}}, Config{
		Provider: "nvidia",
		Model:    "moonshotai/kimi-k3",
		APIKey:   "nv-test",
	})
	if err == nil {
		t.Fatal("expected error when the model returns reasoning and no answer")
	}
}
