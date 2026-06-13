package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"stealth-assist/llm"
)

type AskRequest struct {
	Messages []llm.Message `json:"messages"`
}

type AskResponse struct {
	Reply string `json:"reply"`
}

func enableCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	}
}

func handleAsk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Messages) == 0 {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	reply, err := llm.AskLLM(req.Messages)
	if err != nil {
		log.Printf("LLM error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AskResponse{Reply: reply})
}

func main() {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		log.Println("Warning: ANTHROPIC_API_KEY is not set")
	}

	http.HandleFunc("/api/ask", enableCORS(handleAsk))

	log.Println("Listening on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
