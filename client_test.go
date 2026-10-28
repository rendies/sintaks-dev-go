package sintaksdev

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeGateway records the last request and replies with handler.
type fakeGateway struct {
	*httptest.Server
	calls   int
	lastReq *http.Request
	body    map[string]any
}

func newFakeGateway(t *testing.T, handler http.HandlerFunc) *fakeGateway {
	t.Helper()
	g := &fakeGateway{}
	g.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.calls++
		g.lastReq = r
		g.body = nil
		_ = json.NewDecoder(r.Body).Decode(&g.body)
		w.Header().Set("X-Request-ID", "req_test")
		handler(w, r)
	}))
	t.Cleanup(g.Close)
	return g
}

func (g *fakeGateway) client(opts ...Option) *Client {
	return New("sk-test", append([]Option{WithBaseURL(g.URL + "/")}, opts...)...)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func TestChatCompletion(t *testing.T) {
	g := newFakeGateway(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 200, map[string]any{
			"id": "chatcmpl-1", "object": "chat.completion", "created": 1, "model": "claude-sonnet-5-5",
			"choices": []any{map[string]any{"index": 0, "message": map[string]any{"role": "assistant", "content": "Halo"}, "finish_reason": "stop"}},
			"usage":   map[string]any{"prompt_tokens": 3, "completion_tokens": 1, "total_tokens": 4},
		})
	})
	resp, err := g.client().ChatCompletion(context.Background(), ChatRequest{
		Messages:  []Message{{Role: RoleUser, Content: "Hai"}},
		MaxTokens: Ptr(10),
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content() != "Halo" || resp.Choices[0].FinishReason != "stop" || resp.Usage.TotalTokens != 4 || resp.RequestID != "req_test" {
		t.Fatalf("resp = %+v", resp)
	}
	r := g.lastReq
	if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer sk-test" ||
		r.Header.Get("Content-Type") != "application/json" || r.Header.Get("Accept") != "application/json" || !strings.HasPrefix(r.Header.Get("User-Agent"), "sintaksdev-go/") {
		t.Fatalf("request = %s %s %v", r.Method, r.URL.Path, r.Header)
	}
	if g.body["model"] != DefaultChatModel || g.body["stream"] != false || g.body["max_tokens"] != float64(10) || g.body["stream_options"] != nil || g.body["temperature"] != nil {
		t.Fatalf("body = %v", g.body)
	}
}

func TestClientModelOptions(t *testing.T) {
	g := newFakeGateway(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/images/generations" {
			writeJSON(w, 200, map[string]any{"data": []any{}})
			return
		}
		writeJSON(w, 200, map[string]any{"choices": []any{}})
	})
	c := g.client(WithChatModel("chat-x"), WithImageModel("img-x"), WithHTTPClient(&http.Client{}))
	if resp, err := c.ChatCompletion(context.Background(), ChatRequest{}); err != nil || g.body["model"] != "chat-x" || resp.Content() != "" {
		t.Fatalf("model = %v err = %v", g.body["model"], err)
	}
	if _, err := c.ChatCompletion(context.Background(), ChatRequest{Model: "explicit"}); err != nil || g.body["model"] != "explicit" {
		t.Fatalf("model = %v err = %v", g.body["model"], err)
	}
	if _, err := c.GenerateImage(context.Background(), ImageRequest{Prompt: "p"}); err != nil || g.body["model"] != "img-x" {
		t.Fatalf("model = %v err = %v", g.body["model"], err)
	}
	if New("k").baseURL != DefaultBaseURL {
		t.Fatal("default base URL")
	}
}

func sse(w http.ResponseWriter, events ...string) {
	w.Header().Set("Content-Type", "text/event-stream")
	for _, e := range events {
		fmt.Fprintf(w, "data: %s\n\n", e)
		w.(http.Flusher).Flush()
	}
}

func TestChatCompletionStream(t *testing.T) {
	g := newFakeGateway(t, func(w http.ResponseWriter, _ *http.Request) {
		sse(w,
			`{"id":"c","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}`,
			`{"id":"c","choices":[{"index":0,"delta":{"content":"Selamat"},"finish_reason":null}]}`,
			`{"id":"c","choices":[{"index":0,"delta":{"content":" pagi"},"finish_reason":null}]}`,
			`{"id":"c","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
			`{"id":"c","choices":[],"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}`,
			`[DONE]`)
	})
	stream, err := g.client().ChatCompletionStream(context.Background(), ChatRequest{Messages: []Message{{Role: RoleUser, Content: "pagi"}}})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	var text strings.Builder
	var finish string
	var usage *Usage
	for {
		ch, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		text.WriteString(ch.Content())
		if len(ch.Choices) > 0 && ch.Choices[0].FinishReason != "" {
			finish = ch.Choices[0].FinishReason
		}
		if ch.Usage != nil {
			usage = ch.Usage
		}
	}
	if text.String() != "Selamat pagi" || finish != "stop" || usage == nil || usage.TotalTokens != 7 || stream.RequestID() != "req_test" {
		t.Fatalf("text %q finish %q usage %+v", text.String(), finish, usage)
	}
	if _, err := stream.Recv(); !errors.Is(err, io.EOF) {
		t.Fatalf("Recv after DONE = %v", err)
	}
	so, _ := g.body["stream_options"].(map[string]any)
	if g.body["stream"] != true || so["include_usage"] != true || g.lastReq.Header.Get("Accept") != "text/event-stream" {
		t.Fatalf("body = %v", g.body)
	}
	if (*ChatChunk)(nil).Content() != "" || (&ChatChunk{}).Content() != "" {
		t.Fatal("empty chunk content")
	}
}

func TestChatStreamErrors(t *testing.T) {
	cases := map[string]struct {
		events []string
		check  func(error) bool
	}{
		"mid-stream error": {[]string{`{"choices":[{"delta":{"content":"a"}}]}`, `{"error":{"message":"throttled","type":"upstream_error","code":null}}`}, func(err error) bool {
			var e *APIError
			return errors.As(err, &e) && e.StatusCode == 0 && e.Type == "upstream_error" && e.Message == "throttled" && e.RequestID == "req_test" &&
				e.Error() == "sintaks: upstream_error: throttled (request_id req_test)"
		}},
		"no DONE":   {[]string{`{"choices":[{"delta":{"content":"a"}}]}`}, func(err error) bool { return errors.Is(err, io.ErrUnexpectedEOF) }},
		"bad chunk": {[]string{`{"choices":[{"delta":{"content":"a"}}]}`, `{not json`}, func(err error) bool { return err != nil && strings.Contains(err.Error(), "decode stream chunk") }},
	}
	for name, tc := range cases {
		g := newFakeGateway(t, func(w http.ResponseWriter, _ *http.Request) {
			fmt.Fprint(w, ": comment\nevent: message\n")
			sse(w, tc.events...)
		})
		stream, err := g.client().ChatCompletionStream(context.Background(), ChatRequest{})
		if err != nil {
			t.Fatal(err)
		}
		if ch, err := stream.Recv(); err != nil || ch.Content() != "a" {
			t.Fatalf("%s: first chunk %v %v", name, ch, err)
		}
		if _, err := stream.Recv(); !tc.check(err) {
			t.Errorf("%s: err = %v", name, err)
		}
		_ = stream.Close()
	}
}

func TestAPIErrors(t *testing.T) {
	cases := []struct {
		status int
		body   string
		typ    string
		code   string
		is     func(error) bool
		retry  bool
	}{
		{401, `{"error":{"message":"invalid API key","type":"invalid_request_error","code":"invalid_api_key"}}`, "invalid_request_error", "invalid_api_key", IsUnauthorized, false},
		{402, `{"error":{"message":"insufficient credits","type":"insufficient_quota","code":"insufficient_quota"}}`, "insufficient_quota", "insufficient_quota", IsInsufficientMicrocredits, false},
		{400, `{"error":{"message":"bad size","type":"invalid_request_error","code":null}}`, "invalid_request_error", "", IsInvalidRequest, false},
		{429, `{"error":{"message":"slow down","type":"rate_limit_error"}}`, "rate_limit_error", "", IsRateLimited, true},
		{503, `{"error":{"message":"down","type":"upstream_error"}}`, "upstream_error", "", IsRetryable, true},
		{504, `{"error":{"message":"timeout","type":"upstream_error"}}`, "upstream_error", "", IsRetryable, true},
		{502, `<html>bad gateway</html>`, "", "", func(err error) bool { return strings.Contains(err.Error(), "502 <html>bad gateway</html>") }, false},
		{500, ``, "", "", func(err error) bool { return strings.Contains(err.Error(), "Internal Server Error") }, false},
	}
	for _, tc := range cases {
		g := newFakeGateway(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(tc.status)
			fmt.Fprint(w, tc.body)
		})
		_, err := g.client().GenerateImage(context.Background(), ImageRequest{Prompt: "p"})
		var e *APIError
		if !errors.As(err, &e) || e.StatusCode != tc.status || e.Type != tc.typ || e.Code != tc.code || e.RequestID != "req_test" || !tc.is(err) || IsRetryable(err) != tc.retry {
			t.Errorf("%d: err = %#v", tc.status, err)
		}
		if _, err := g.client().ChatCompletionStream(context.Background(), ChatRequest{}); statusOf(err) != tc.status {
			t.Errorf("%d: stream err = %v", tc.status, err)
		}
	}
	if IsUnauthorized(errors.New("x")) || IsRetryable(nil) {
		t.Fatal("non-API errors")
	}
}

func TestGenerateImage(t *testing.T) {
	g := newFakeGateway(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 200, map[string]any{"created": 7, "data": []any{map[string]any{"url": "https://is3.cloudhost.id/b/a.png"}, map[string]any{"url": "https://is3.cloudhost.id/b/b.png"}}})
	})
	resp, err := g.client().GenerateImage(context.Background(), ImageRequest{Prompt: "kucing", N: 2, Size: "512x512"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Created != 7 || len(resp.Data) != 2 || resp.Data[1].URL != "https://is3.cloudhost.id/b/b.png" || resp.RequestID != "req_test" {
		t.Fatalf("resp = %+v", resp)
	}
	if g.lastReq.URL.Path != "/v1/images/generations" || g.body["prompt"] != "kucing" || g.body["n"] != float64(2) || g.body["size"] != "512x512" || g.body["model"] != DefaultImageModel {
		t.Fatalf("body = %v", g.body)
	}
}

func TestTransportAndDecodeErrors(t *testing.T) {
	g := newFakeGateway(t, func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "not json") })
	if _, err := g.client().ChatCompletion(context.Background(), ChatRequest{}); err == nil || !strings.Contains(err.Error(), "decode response (request_id req_test)") {
		t.Fatalf("err = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := g.client().ChatCompletion(ctx, ChatRequest{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	if _, err := New("k", WithBaseURL("http://127.0.0.1:1")).GenerateImage(context.Background(), ImageRequest{}); err == nil || statusOf(err) != 0 {
		t.Fatalf("err = %v", err)
	}
	if _, err := New("k", WithBaseURL("://bad")).GenerateImage(context.Background(), ImageRequest{}); err == nil {
		t.Fatal("expected bad URL error")
	}
}

func TestEmbedding(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/embeddings" {
			t.Errorf("path = %q", r.URL.Path)
		}
		var req EmbeddingRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if req.Model != "qwen3-embedding-0-6b" || len(req.Input) != 2 {
			t.Errorf("body = %+v", req)
		}
		w.Header().Set("X-Request-ID", "req_emb_test")
		_, _ = io.WriteString(w, `{"object":"list","data":[{"object":"embedding","index":0,"embedding":[0.1,0.2]},{"object":"embedding","index":1,"embedding":[0.3,0.4]}],"model":"qwen3-embedding-0-6b","usage":{"prompt_tokens":7,"total_tokens":7}}`)
	}))
	defer srv.Close()

	client := New("sk-test", WithBaseURL(srv.URL))
	resp, err := client.Embedding(context.Background(), EmbeddingRequest{Input: []string{"a", "b"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Data) != 2 || resp.Data[0].Embedding[1] != 0.2 || resp.Usage.PromptTokens != 7 {
		t.Fatalf("resp = %+v", resp)
	}
	if resp.RequestID != "req_emb_test" {
		t.Fatalf("request id = %q", resp.RequestID)
	}
}
