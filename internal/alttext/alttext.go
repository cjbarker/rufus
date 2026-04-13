// Package alttext generates image alt-text keywords via an OpenAI-compatible
// LLM vision API.
package alttext

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	_ "golang.org/x/image/bmp"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

const (
	// Prompt instructs the LLM to produce W3C-compliant alt-text keywords.
	Prompt = `Describe what's in this picture and then reduce the description to the W3C specification text string length for an HTML image alt tags attribute. Description should include the subject, environment, settings, and the overall mood of the image. Respond with a list of keywords 10 or less.`

	maxKeywords = 10
	maxPixels   = 512
	jpegQuality = 85
)

// Request configures a single alt-text generation call.
type Request struct {
	ImagePath string
	ApiURL    string
	ApiKey    string
	Model     string
	Timeout   time.Duration
}

// supportedMIME maps image file extensions to MIME types.
var supportedMIME = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
	".bmp":  "image/bmp",
	".tiff": "image/tiff",
	".tif":  "image/tiff",
	".webp": "image/webp",
}

// MimeForExt returns the MIME type for a supported image file extension.
// The extension should include the leading dot (e.g. ".jpg").
func MimeForExt(ext string) (string, bool) {
	mime, ok := supportedMIME[strings.ToLower(ext)]
	return mime, ok
}

// Generate sends the image at req.ImagePath to an OpenAI-compatible LLM
// vision API and returns the raw text response.
func Generate(ctx context.Context, req Request) (string, error) {
	if req.ApiKey == "" {
		return "", fmt.Errorf("API key is required")
	}
	if req.ApiURL == "" {
		return "", fmt.Errorf("API URL is required")
	}

	ext := strings.ToLower(filepath.Ext(req.ImagePath))
	if _, ok := MimeForExt(ext); !ok {
		return "", fmt.Errorf("unsupported image format %q", ext)
	}

	imgBytes, err := resizeImage(req.ImagePath)
	if err != nil {
		return "", fmt.Errorf("preparing image: %w", err)
	}

	b64 := base64.StdEncoding.EncodeToString(imgBytes)
	dataURL := "data:image/jpeg;base64," + b64

	opts := []option.RequestOption{
		option.WithAPIKey(req.ApiKey),
		option.WithBaseURL(req.ApiURL),
	}
	if req.Timeout > 0 {
		opts = append(opts, option.WithRequestTimeout(req.Timeout))
	}
	client := openai.NewClient(opts...)

	completion, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:               req.Model,
		MaxCompletionTokens: openai.Int(300),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage([]openai.ChatCompletionContentPartUnionParam{
				openai.TextContentPart(Prompt),
				openai.ImageContentPart(openai.ChatCompletionContentPartImageImageURLParam{
					URL:    dataURL,
					Detail: "auto",
				}),
			}),
		},
	})
	if err != nil {
		return "", fmt.Errorf("LLM API request: %w", err)
	}

	if len(completion.Choices) == 0 {
		return "", fmt.Errorf("LLM API returned no choices")
	}
	return completion.Choices[0].Message.Content, nil
}

// resizeImage decodes the image at filePath, scales it to fit within
// 512x512 pixels (preserving aspect ratio), and re-encodes as JPEG.
func resizeImage(filePath string) ([]byte, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("opening image: %w", err)
	}
	defer func() { _ = f.Close() }()

	src, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decoding image: %w", err)
	}

	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	// Only resize if the image exceeds the max dimension.
	if w > maxPixels || h > maxPixels {
		var newW, newH int
		if w >= h {
			newW = maxPixels
			newH = h * maxPixels / w
		} else {
			newH = maxPixels
			newW = w * maxPixels / h
		}
		if newW < 1 {
			newW = 1
		}
		if newH < 1 {
			newH = 1
		}

		dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
		draw.CatmullRom.Scale(dst, dst.Bounds(), src, bounds, draw.Over, nil)
		src = dst
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, src, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return nil, fmt.Errorf("encoding resized image: %w", err)
	}
	return buf.Bytes(), nil
}

// numberPrefix matches leading numbering like "1.", "2)", "10.", etc.
var numberPrefix = regexp.MustCompile(`^\d+[.)]\s*`)

// ParseKeywords extracts individual keywords from the LLM response text.
// It handles comma-separated lists, numbered lists, and bulleted lists.
func ParseKeywords(raw string) []string {
	seen := make(map[string]bool)
	var keywords []string

	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Strip leading bullets.
		line = strings.TrimLeft(line, "-*•–— ")
		// Strip leading numbering.
		line = numberPrefix.ReplaceAllString(line, "")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Split on commas if present.
		parts := strings.Split(line, ",")
		for _, part := range parts {
			kw := strings.ToLower(strings.TrimSpace(part))
			// Strip surrounding quotes.
			kw = strings.Trim(kw, "\"'`")
			kw = strings.TrimSpace(kw)
			if kw == "" {
				continue
			}
			if !seen[kw] {
				seen[kw] = true
				keywords = append(keywords, kw)
			}
			if len(keywords) >= maxKeywords {
				return keywords
			}
		}
	}
	return keywords
}
