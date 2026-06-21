package llm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
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
	if cfg.Model != "gemini-2.0-flash" {
		t.Errorf("model: got %q, want %q", cfg.Model, "gemini-2.0-flash")
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
