package sintaksdev

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

const (
	// DefaultSlugMaxLength bounds slugs from GenerateSlug.
	DefaultSlugMaxLength = 60
	// DefaultOGCardSize is close to the 1200x630 Open Graph standard (about
	// 1.9:1). The gateway maps it to the 16:9 aspect ratio on Stable Image Core,
	// and it is also a valid Nova Canvas size (both sides multiples of 16).
	DefaultOGCardSize = "1216x640"
	// DefaultOGCardStyle is used when OGCardRequest.Style is empty.
	DefaultOGCardStyle = "clean, modern, professional, soft vibrant gradients"

	slugMaxTokens   = 32
	slugTemperature = 0.2
	maxTitleRunes   = 150
	maxDescRunes    = 300
)

const slugSystemPrompt = "You write SEO-friendly URL slugs. Reply with exactly one slug and nothing else: " +
	"lowercase ASCII letters, digits and hyphens only, no spaces, quotes or explanation. " +
	"Keep the most important keywords in the language of the title and drop filler words where it stays readable."

// SlugRequest describes the page to generate a slug for.
type SlugRequest struct {
	Title       string
	Description string
	// MaxLength defaults to DefaultSlugMaxLength.
	MaxLength int
}

// GenerateSlug asks the chat model for an SEO slug. The reply is normalized
// with Slugify, so the result is always lowercase ASCII words joined by
// hyphens and at most MaxLength characters; if the model's reply has no
// usable characters the title itself is slugified.
func (c *Client) GenerateSlug(ctx context.Context, req SlugRequest) (string, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return "", errors.New("sintaks: SlugRequest.Title is required")
	}
	maxLen := req.MaxLength
	if maxLen <= 0 {
		maxLen = DefaultSlugMaxLength
	}
	prompt := "Title: " + truncateRunes(title, maxTitleRunes)
	if d := strings.TrimSpace(req.Description); d != "" {
		prompt += "\nDescription: " + truncateRunes(d, maxDescRunes)
	}
	prompt += fmt.Sprintf("\nMaximum length: %d characters", maxLen)

	resp, err := c.ChatCompletion(ctx, ChatRequest{
		Messages: []Message{
			{Role: RoleSystem, Content: slugSystemPrompt},
			{Role: RoleUser, Content: prompt},
		},
		MaxTokens:   Ptr(slugMaxTokens),
		Temperature: Ptr(slugTemperature),
	})
	if err != nil {
		return "", err
	}
	if slug := Slugify(extractSlug(resp.Content()), maxLen); slug != "" {
		return slug, nil
	}
	if slug := Slugify(title, maxLen); slug != "" {
		return slug, nil
	}
	return "", fmt.Errorf("sintaks: could not derive a slug (request_id %s)", resp.RequestID)
}

// OGCardRequest describes a social preview (Open Graph) card.
type OGCardRequest struct {
	Title       string
	Description string
	// Style defaults to DefaultOGCardStyle.
	Style string
	// Size defaults to DefaultOGCardSize.
	Size string
}

// GenerateOGCard generates the background artwork for an Open Graph card and
// returns its hosted (IDCloudHost S3) URL. Image models render text poorly,
// so the image deliberately contains no text; overlay the title yourself.
func (c *Client) GenerateOGCard(ctx context.Context, req OGCardRequest) (string, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return "", errors.New("sintaks: OGCardRequest.Title is required")
	}
	size := req.Size
	if size == "" {
		size = DefaultOGCardSize
	}
	resp, err := c.GenerateImage(ctx, ImageRequest{Prompt: ogCardPrompt(title, req.Description, req.Style), N: 1, Size: size})
	if err != nil {
		return "", err
	}
	if len(resp.Data) == 0 || resp.Data[0].URL == "" {
		return "", fmt.Errorf("sintaks: no image URL returned (request_id %s)", resp.RequestID)
	}
	return resp.Data[0].URL, nil
}

func ogCardPrompt(title, desc, style string) string {
	if style = strings.TrimSpace(style); style == "" {
		style = DefaultOGCardStyle
	}
	var b strings.Builder
	b.WriteString("Background artwork for a social media link preview card about: ")
	b.WriteString(truncateRunes(title, maxTitleRunes))
	b.WriteString(".")
	if d := strings.TrimSpace(desc); d != "" {
		b.WriteString(" Context: " + truncateRunes(d, maxDescRunes) + ".")
	}
	b.WriteString(" Style: " + truncateRunes(style, maxTitleRunes) + ".")
	b.WriteString(" Wide composition with calm negative space for a headline overlay. No text, no letters, no logos, no watermarks.")
	return b.String()
}

// extractSlug picks the slug out of a chat reply that may wrap it in prose,
// quotes or Markdown, e.g. "Slug: `jual-sepatu`".
func extractSlug(reply string) string {
	lines := strings.Split(strings.TrimSpace(reply), "\n")
	line := lines[len(lines)-1]
	if i := strings.LastIndex(line, ":"); i >= 0 {
		line = line[i+1:]
	}
	return strings.Trim(line, " \t\"'`*")
}

var latinFold = func() map[rune]rune {
	m := map[rune]rune{}
	for base, variants := range map[rune]string{
		'a': "àáâãäåā", 'c': "çć", 'e': "èéêëē", 'i': "ìíîïī", 'n': "ñń",
		'o': "òóôõöøō", 'u': "ùúûüū", 'y': "ýÿ", 's': "śš", 'z': "źżž",
	} {
		for _, v := range variants {
			m[v] = base
		}
	}
	return m
}()

// Slugify lowercases s, folds common Latin accents to ASCII, joins runs of
// a-z0-9 with single hyphens and truncates to maxLen (if > 0) at a word
// boundary where possible.
func Slugify(s string, maxLen int) string {
	var b strings.Builder
	pendingHyphen := false
	for _, r := range strings.ToLower(s) {
		if f, ok := latinFold[r]; ok {
			r = f
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			if pendingHyphen && b.Len() > 0 {
				b.WriteByte('-')
			}
			pendingHyphen = false
			b.WriteRune(r)
			continue
		}
		pendingHyphen = true
	}
	out := b.String()
	if maxLen > 0 && len(out) > maxLen {
		if out[maxLen] == '-' {
			out = out[:maxLen]
		} else if i := strings.LastIndexByte(out[:maxLen], '-'); i > 0 {
			out = out[:i]
		} else {
			out = out[:maxLen]
		}
	}
	return out
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
