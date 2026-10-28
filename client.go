// Package sintaksdev is a Go client for the sintaks.dev OpenAI-compatible
// API gateway. It depends only on the standard library.
package sintaksdev

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

const (
	// DefaultBaseURL is the production gateway; override it with WithBaseURL.
	DefaultBaseURL = "https://api.sintaks.dev"
	// DefaultChatModel and DefaultImageModel are sent when a request leaves
	// Model empty. The gateway maps them to its configured upstream models.
	DefaultChatModel      = "gpt-5-5"
	DefaultImageModel     = "gpt-5-5"
	DefaultEmbeddingModel = "qwen3-embedding-0-6b"

	userAgent = "sintaksdev-go/1"
)

// Client calls the sintaks.dev API. It is safe for concurrent use.
type Client struct {
	apiKey         string
	baseURL        string
	httpClient     *http.Client
	chatModel      string
	imageModel     string
	embeddingModel string
}

type Option func(*Client)

// WithBaseURL sets the gateway URL, e.g. "https://api.sintaks.dev".
func WithBaseURL(u string) Option {
	return func(c *Client) { c.baseURL = strings.TrimRight(u, "/") }
}

// WithHTTPClient sets the HTTP client. Avoid http.Client.Timeout with
// streaming; bound requests with the context instead.
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) { c.httpClient = h }
}

func WithChatModel(model string) Option {
	return func(c *Client) { c.chatModel = model }
}

func WithImageModel(model string) Option {
	return func(c *Client) { c.imageModel = model }
}

func WithEmbeddingModel(model string) Option {
	return func(c *Client) { c.embeddingModel = model }
}

// New returns a client authenticating with apiKey.
func New(apiKey string, opts ...Option) *Client {
	c := &Client{
		apiKey:         apiKey,
		baseURL:        DefaultBaseURL,
		httpClient:     &http.Client{},
		chatModel:      DefaultChatModel,
		imageModel:     DefaultImageModel,
		embeddingModel: DefaultEmbeddingModel,
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// ChatCompletion sends a non-streaming chat completion request.
func (c *Client) ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	if req.Model == "" {
		req.Model = c.chatModel
	}
	var out ChatResponse
	id, err := c.do(ctx, "/v1/chat/completions", chatWire{ChatRequest: req}, &out)
	if err != nil {
		return nil, err
	}
	out.RequestID = id
	return &out, nil
}

// ChatCompletionStream sends a streaming chat completion request. The caller
// must Close the returned stream. Token usage is requested and delivered as
// a final chunk with empty Choices.
func (c *Client) ChatCompletionStream(ctx context.Context, req ChatRequest) (*ChatStream, error) {
	if req.Model == "" {
		req.Model = c.chatModel
	}
	resp, err := c.send(ctx, "/v1/chat/completions", chatWire{ChatRequest: req, Stream: true, StreamOptions: &streamOptions{IncludeUsage: true}}, true)
	if err != nil {
		return nil, err
	}
	return newChatStream(resp), nil
}

// GenerateImage generates images and returns their hosted URLs.
func (c *Client) GenerateImage(ctx context.Context, req ImageRequest) (*ImageResponse, error) {
	if req.Model == "" {
		req.Model = c.imageModel
	}
	var out ImageResponse
	id, err := c.do(ctx, "/v1/images/generations", req, &out)
	if err != nil {
		return nil, err
	}
	out.RequestID = id
	return &out, nil
}

// Embedding returns embedding vectors for the input texts, in request order.
func (c *Client) Embedding(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error) {
	if req.Model == "" {
		req.Model = c.embeddingModel
	}
	var out EmbeddingResponse
	id, err := c.do(ctx, "/v1/embeddings", req, &out)
	if err != nil {
		return nil, err
	}
	out.RequestID = id
	return &out, nil
}

func (c *Client) do(ctx context.Context, path string, body, out any) (string, error) {
	resp, err := c.send(ctx, path, body, false)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	id := resp.Header.Get("X-Request-ID")
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return id, fmt.Errorf("sintaks: decode response (request_id %s): %w", id, err)
	}
	return id, nil
}

// send performs the request and returns the response for 2xx statuses;
// other statuses are returned as *APIError.
func (c *Client) send(ctx context.Context, path string, body any, stream bool) (*http.Response, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("sintaks: encode request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("sintaks: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)
	if stream {
		req.Header.Set("Accept", "text/event-stream")
	} else {
		req.Header.Set("Accept", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sintaks: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		defer resp.Body.Close()
		return nil, parseAPIError(resp)
	}
	return resp, nil
}
