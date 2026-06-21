package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"stealth-assist/llm"
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

// withClaudeMock starts a server that returns a Claude-shaped response and
// overrides llm.HTTPClient so handlers hit it. Cleanup restores the original.
func withClaudeMock(t *testing.T, reply string) func() {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"content": []map[string]string{{"type": "text", "text": reply}},
		})
	}))
	orig := llm.HTTPClient
	llm.HTTPClient = &http.Client{Transport: &testTransport{target: srv.URL}}
	return func() {
		llm.HTTPClient = orig
		srv.Close()
	}
}

// ── CORS middleware ───────────────────────────────────────────────────────────

func TestCORSHeaders(t *testing.T) {
	handler := enableCORS(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("Allow-Origin: got %q, want %q", got, "*")
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got != "POST, OPTIONS" {
		t.Errorf("Allow-Methods: got %q, want %q", got, "POST, OPTIONS")
	}
}

func TestCORSPreflight(t *testing.T) {
	handler := enableCORS(func(w http.ResponseWriter, r *http.Request) {
		t.Error("inner handler should not be called for OPTIONS")
	})

	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
}

// ── /api/ask validation ───────────────────────────────────────────────────────

func TestAskMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/ask", nil)
	rec := httptest.NewRecorder()
	handleAsk(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestAskBadJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/ask", bytes.NewBufferString("{bad json"))
	rec := httptest.NewRecorder()
	handleAsk(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAskEmptyMessages(t *testing.T) {
	body, _ := json.Marshal(AskRequest{Messages: []llm.Message{}})
	req := httptest.NewRequest(http.MethodPost, "/api/ask", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handleAsk(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAskSuccess(t *testing.T) {
	cleanup := withClaudeMock(t, "hello world")
	defer cleanup()

	body, _ := json.Marshal(AskRequest{
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
		Provider: "anthropic",
		APIKey:   "test-key",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/ask", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handleAsk(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}

	var resp AskResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Reply != "hello world" {
		t.Errorf("reply: got %q, want %q", resp.Reply, "hello world")
	}
}

// ── /api/screenshot validation ────────────────────────────────────────────────

func TestScreenshotMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/screenshot", nil)
	rec := httptest.NewRecorder()
	handleScreenshot(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestScreenshotBadJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/screenshot", bytes.NewBufferString("{bad json"))
	rec := httptest.NewRecorder()
	handleScreenshot(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestScreenshotEmptyImage(t *testing.T) {
	body, _ := json.Marshal(ScreenshotRequest{Image: ""})
	req := httptest.NewRequest(http.MethodPost, "/api/screenshot", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handleScreenshot(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestScreenshotSuccess(t *testing.T) {
	cleanup := withClaudeMock(t, "I see questions")
	defer cleanup()

	body, _ := json.Marshal(ScreenshotRequest{
		Image:    "base64encodedpng",
		Provider: "anthropic",
		APIKey:   "test-key",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/screenshot", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handleScreenshot(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}

	var resp AskResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Reply != "I see questions" {
		t.Errorf("reply: got %q, want %q", resp.Reply, "I see questions")
	}
}
