package sintaksdev

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ChatStream reads server-sent chat completion chunks.
type ChatStream struct {
	body      io.ReadCloser
	sc        *bufio.Scanner
	requestID string
	done      bool
}

func newChatStream(resp *http.Response) *ChatStream {
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64<<10), 1<<20)
	return &ChatStream{body: resp.Body, sc: sc, requestID: resp.Header.Get("X-Request-ID")}
}

// Recv returns the next chunk, io.EOF after the final "[DONE]" event, or an
// *APIError if the gateway reported an error mid-stream. A stream that ends
// without "[DONE]" returns io.ErrUnexpectedEOF.
func (s *ChatStream) Recv() (*ChatChunk, error) {
	if s.done {
		return nil, io.EOF
	}
	for s.sc.Scan() {
		data, ok := strings.CutPrefix(s.sc.Text(), "data:")
		if !ok {
			continue
		}
		data = strings.TrimSpace(data)
		if data == "[DONE]" {
			s.done = true
			return nil, io.EOF
		}
		var ev struct {
			ChatChunk
			Error *errorBody `json:"error"`
		}
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			return nil, fmt.Errorf("sintaks: decode stream chunk (request_id %s): %w", s.requestID, err)
		}
		if ev.Error != nil {
			s.done = true
			return nil, ev.Error.toAPIError(0, s.requestID)
		}
		return &ev.ChatChunk, nil
	}
	s.done = true
	if err := s.sc.Err(); err != nil {
		return nil, fmt.Errorf("sintaks: read stream (request_id %s): %w", s.requestID, err)
	}
	return nil, io.ErrUnexpectedEOF
}

// RequestID returns the gateway's X-Request-ID for this stream.
func (s *ChatStream) RequestID() string { return s.requestID }

// Close releases the connection. Closing before the end cancels the request
// upstream; tokens generated so far are still billed.
func (s *ChatStream) Close() error { return s.body.Close() }
