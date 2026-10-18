package sintaksdev

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSlugify(t *testing.T) {
	cases := []struct {
		in   string
		max  int
		want string
	}{
		{"Cara Membuat QR Code untuk Menu Restoran", 0, "cara-membuat-qr-code-untuk-menu-restoran"},
		{"  Héllo, Wörld!!  ", 0, "hello-world"},
		{"Jual Sepatu -- Murah   2026", 0, "jual-sepatu-murah-2026"},
		{"Café Crème Brûlée", 0, "cafe-creme-brulee"},
		{"日本語 only", 0, "only"},
		{"!!!", 0, ""},
		{"cara-membuat-qr-code", 12, "cara-membuat"},
		{"cara-membuat-qr-code", 14, "cara-membuat"},
		{"cara-membuat-qr-code", 15, "cara-membuat-qr"},
		{"supercalifragilistic", 5, "super"},
	}
	for _, tc := range cases {
		if got := Slugify(tc.in, tc.max); got != tc.want {
			t.Errorf("Slugify(%q, %d) = %q, want %q", tc.in, tc.max, got, tc.want)
		}
	}
}

func TestExtractSlug(t *testing.T) {
	for in, want := range map[string]string{
		"jual-sepatu-murah":                    "jual-sepatu-murah",
		"Slug: `jual-sepatu-murah`":            "jual-sepatu-murah",
		"Here is a slug:\n\n**jual-sepatu**\n": "jual-sepatu",
		`"jual-sepatu"`:                        "jual-sepatu",
		"":                                     "",
	} {
		if got := extractSlug(in); got != want {
			t.Errorf("extractSlug(%q) = %q, want %q", in, got, want)
		}
	}
}

func chatReply(content string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 200, map[string]any{"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": content}}}})
	}
}

func TestGenerateSlug(t *testing.T) {
	g := newFakeGateway(t, chatReply("Slug: `Cara Membuat QR Code Menu Restoran`"))
	slug, err := g.client().GenerateSlug(context.Background(), SlugRequest{Title: " Cara Membuat QR Code untuk Menu Restoran ", Description: "Panduan lengkap"})
	if err != nil || slug != "cara-membuat-qr-code-menu-restoran" {
		t.Fatalf("slug = %q, err = %v", slug, err)
	}
	if g.body["max_tokens"] != float64(slugMaxTokens) || g.body["temperature"] != slugTemperature || g.body["model"] != DefaultChatModel {
		t.Fatalf("body = %v", g.body)
	}
	msgs := g.body["messages"].([]any)
	sys, user := msgs[0].(map[string]any), msgs[1].(map[string]any)
	if sys["role"] != "system" || !strings.Contains(sys["content"].(string), "slug") || user["role"] != "user" ||
		user["content"] != "Title: Cara Membuat QR Code untuk Menu Restoran\nDescription: Panduan lengkap\nMaximum length: 60 characters" {
		t.Fatalf("messages = %v", msgs)
	}
}

func TestGenerateSlugMaxLengthAndFallback(t *testing.T) {
	g := newFakeGateway(t, chatReply("cara-membuat-qr-code-menu-restoran"))
	if slug, _ := g.client().GenerateSlug(context.Background(), SlugRequest{Title: "x", MaxLength: 16}); slug != "cara-membuat-qr" {
		t.Fatalf("slug = %q", slug)
	}
	if !strings.HasSuffix(g.body["messages"].([]any)[1].(map[string]any)["content"].(string), "Maximum length: 16 characters") {
		t.Fatalf("prompt = %v", g.body["messages"])
	}

	g = newFakeGateway(t, chatReply("???"))
	if slug, err := g.client().GenerateSlug(context.Background(), SlugRequest{Title: "Promo Akhir Tahun"}); err != nil || slug != "promo-akhir-tahun" {
		t.Fatalf("fallback slug = %q, err = %v", slug, err)
	}
	if _, err := g.client().GenerateSlug(context.Background(), SlugRequest{Title: "日本語"}); err == nil || !strings.Contains(err.Error(), "request_id req_test") {
		t.Fatalf("err = %v", err)
	}
}

func TestGenerateSlugErrors(t *testing.T) {
	g := newFakeGateway(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 402, map[string]any{"error": map[string]any{"message": "insufficient credits", "type": "insufficient_quota"}})
	})
	if _, err := g.client().GenerateSlug(context.Background(), SlugRequest{Title: " "}); err == nil || g.calls != 0 {
		t.Fatalf("empty title: err = %v calls = %d", err, g.calls)
	}
	if _, err := g.client().GenerateSlug(context.Background(), SlugRequest{Title: "t"}); !IsInsufficientMicrocredits(err) {
		t.Fatalf("err = %v", err)
	}
}

func TestGenerateOGCard(t *testing.T) {
	g := newFakeGateway(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 200, map[string]any{"data": []any{map[string]any{"url": "https://is3.cloudhost.id/media/og.png"}}})
	})
	url, err := g.client().GenerateOGCard(context.Background(), OGCardRequest{Title: "Menu Digital", Description: "QR untuk restoran"})
	if err != nil || url != "https://is3.cloudhost.id/media/og.png" {
		t.Fatalf("url = %q, err = %v", url, err)
	}
	p := g.body["prompt"].(string)
	if g.body["size"] != DefaultOGCardSize || g.body["n"] != float64(1) || !strings.Contains(p, "Menu Digital") || !strings.Contains(p, "QR untuk restoran") ||
		!strings.Contains(p, DefaultOGCardStyle) || !strings.Contains(p, "No text") {
		t.Fatalf("body = %v", g.body)
	}
	if _, err := g.client().GenerateOGCard(context.Background(), OGCardRequest{Title: "t", Style: "flat pastel", Size: "1024x512"}); err != nil ||
		g.body["size"] != "1024x512" || !strings.Contains(g.body["prompt"].(string), "flat pastel") {
		t.Fatalf("body = %v err = %v", g.body, err)
	}
}

func TestGenerateOGCardErrors(t *testing.T) {
	g := newFakeGateway(t, func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, 200, map[string]any{"data": []any{}}) })
	if _, err := g.client().GenerateOGCard(context.Background(), OGCardRequest{}); err == nil || g.calls != 0 {
		t.Fatalf("empty title: err = %v", err)
	}
	if _, err := g.client().GenerateOGCard(context.Background(), OGCardRequest{Title: "t"}); err == nil || !strings.Contains(err.Error(), "no image URL") {
		t.Fatalf("err = %v", err)
	}
	g = newFakeGateway(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 400, map[string]any{"error": map[string]any{"message": "bad size", "type": "invalid_request_error"}})
	})
	if _, err := g.client().GenerateOGCard(context.Background(), OGCardRequest{Title: "t", Size: "1200x630"}); !IsInvalidRequest(err) {
		t.Fatalf("err = %v", err)
	}
}

// Keep the prompt within Nova Canvas's 1024-character limit, the strictest supported image model.
func TestOGCardPromptFitsNovaLimit(t *testing.T) {
	long := strings.Repeat("é", 5000)
	if n := utf8.RuneCountInString(ogCardPrompt(long, long, long)); n > 1024 {
		t.Fatalf("prompt is %d runes", n)
	}
}
