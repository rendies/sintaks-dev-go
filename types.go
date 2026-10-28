package sintaksdev

// Message roles.
const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest is an OpenAI-compatible chat completion request. Optional
// fields are pointers; use Ptr to set them.
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   *int      `json:"max_tokens,omitempty"`
	Temperature *float64  `json:"temperature,omitempty"`
	TopP        *float64  `json:"top_p,omitempty"`
	Stop        []string  `json:"stop,omitempty"`
}

type chatWire struct {
	ChatRequest
	Stream        bool           `json:"stream"`
	StreamOptions *streamOptions `json:"stream_options,omitempty"`
}

type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ChatResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   *Usage   `json:"usage"`
	// RequestID is the gateway's X-Request-ID, used in its billing logs.
	RequestID string `json:"-"`
}

// Content returns the first choice's message content.
func (r *ChatResponse) Content() string {
	if r == nil || len(r.Choices) == 0 {
		return ""
	}
	return r.Choices[0].Message.Content
}

type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

type ChatChunk struct {
	ID      string        `json:"id"`
	Created int64         `json:"created"`
	Model   string        `json:"model"`
	Choices []ChunkChoice `json:"choices"`
	Usage   *Usage        `json:"usage"`
}

// Content returns the first choice's delta content.
func (c *ChatChunk) Content() string {
	if c == nil || len(c.Choices) == 0 {
		return ""
	}
	return c.Choices[0].Delta.Content
}

type ChunkChoice struct {
	Index        int    `json:"index"`
	Delta        Delta  `json:"delta"`
	FinishReason string `json:"finish_reason"`
}

type Delta struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ImageRequest is an OpenAI-compatible image generation request. Size is
// "WIDTHxHEIGHT" (default "1024x1024").
type ImageRequest struct {
	Prompt string `json:"prompt"`
	Model  string `json:"model,omitempty"`
	N      int    `json:"n,omitempty"`
	Size   string `json:"size,omitempty"`
}

type ImageResponse struct {
	Created   int64   `json:"created"`
	Data      []Image `json:"data"`
	RequestID string  `json:"-"`
}

type Image struct {
	URL string `json:"url"`
}

// EmbeddingRequest is an OpenAI-compatible embeddings request. Input accepts
// one or more texts; EncodingFormat is "float" (the default) when empty.
type EmbeddingRequest struct {
	Model          string   `json:"model"`
	Input          []string `json:"input"`
	EncodingFormat string   `json:"encoding_format,omitempty"`
	Dimensions     *int     `json:"dimensions,omitempty"`
}

type EmbeddingResponse struct {
	Object    string          `json:"object"`
	Data      []EmbeddingData `json:"data"`
	Model     string          `json:"model"`
	Usage     *EmbeddingUsage `json:"usage"`
	RequestID string          `json:"-"`
}

type EmbeddingData struct {
	Object    string    `json:"object"`
	Index     int       `json:"index"`
	Embedding []float64 `json:"embedding"`
}

type EmbeddingUsage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// Ptr returns a pointer to v, for optional request fields.
func Ptr[T any](v T) *T { return &v }
