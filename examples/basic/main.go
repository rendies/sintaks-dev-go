// Command basic shows how an application uses pkg/sintaksdev to generate
// an SEO slug and an Open Graph card image.
//
//	go run ./examples/basic                             # against a built-in mock gateway
//	SINTAKS_API_KEY=sk-... SINTAKS_BASE_URL=https://api.sintaks.dev go run ./examples/basic
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	"github.com/rendies/sintaks-dev-go"
)

func main() {
	apiKey, baseURL := os.Getenv("SINTAKS_API_KEY"), os.Getenv("SINTAKS_BASE_URL")
	if apiKey == "" {
		mock := httptest.NewServer(mockGateway())
		defer mock.Close()
		apiKey, baseURL = "sk-mock", mock.URL
		fmt.Println("SINTAKS_API_KEY not set; using a mock gateway at", mock.URL)
	}
	var opts []sintaksdev.Option
	if baseURL != "" {
		opts = append(opts, sintaksdev.WithBaseURL(baseURL))
	}
	client := sintaksdev.New(apiKey, opts...)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	slug, err := client.GenerateSlug(ctx, sintaksdev.SlugRequest{
		Title:       "Cara Membuat QR Code untuk Menu Restoran",
		Description: "Panduan membuat QR code menu digital yang bisa dipindai pelanggan.",
	})
	if err != nil {
		fail("GenerateSlug", err)
	}
	fmt.Println("slug:   ", slug)

	ogURL, err := client.GenerateOGCard(ctx, sintaksdev.OGCardRequest{
		Title:       "Cara Membuat QR Code untuk Menu Restoran",
		Description: "restaurant table with a QR code menu",
		Style:       "warm, friendly, flat illustration",
	})
	if err != nil {
		fail("GenerateOGCard", err)
	}
	fmt.Println("og card:", ogURL)
}

func fail(op string, err error) {
	switch {
	case sintaksdev.IsInsufficientMicrocredits(err):
		log.Fatalf("%s: out of credits, top up the account: %v", op, err)
	case sintaksdev.IsUnauthorized(err):
		log.Fatalf("%s: check SINTAKS_API_KEY: %v", op, err)
	default:
		log.Fatalf("%s: %v", op, err)
	}
}

// mockGateway imitates the two sintaks.dev endpoints the example uses.
func mockGateway() http.Handler {
	reply := func(w http.ResponseWriter, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-ID", "req_mock")
		_ = json.NewEncoder(w).Encode(v)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/chat/completions", func(w http.ResponseWriter, _ *http.Request) {
		reply(w, map[string]any{
			"id": "chatcmpl-mock", "object": "chat.completion", "created": time.Now().Unix(), "model": "claude-sonnet-5-5",
			"choices": []any{map[string]any{"index": 0, "finish_reason": "stop",
				"message": map[string]any{"role": "assistant", "content": "cara-membuat-qr-code-menu-restoran"}}},
		})
	})
	mux.HandleFunc("POST /v1/images/generations", func(w http.ResponseWriter, _ *http.Request) {
		reply(w, map[string]any{"created": time.Now().Unix(), "data": []any{
			map[string]any{"url": "https://is3.cloudhost.id/sintaks-media/images/2026/10/04/mock-og-card.png"}}})
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-mock" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"message":"invalid API key","type":"invalid_request_error","code":"invalid_api_key"}}`))
			return
		}
		mux.ServeHTTP(w, r)
	})
}
