package alttext

import (
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// makeTestJPEG creates a JPEG image of the given dimensions in dir and returns its path.
func makeTestJPEG(t *testing.T, dir, name string, w, h int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 128, A: 255})
		}
	}
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	return path
}

// fakeLLMServer returns an httptest server that mimics the OpenAI chat completions API.
func fakeLLMServer(t *testing.T, responseContent string, statusCode int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !strings.Contains(r.Header.Get("Authorization"), "Bearer") {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)

		if statusCode != http.StatusOK {
			resp := map[string]interface{}{
				"error": map[string]string{
					"message": "server error",
					"type":    "server_error",
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		resp := map[string]interface{}{
			"id":      "chatcmpl-test",
			"object":  "chat.completion",
			"created": 1700000000,
			"model":   "gpt-4o",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]string{
						"role":    "assistant",
						"content": responseContent,
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]int{
				"prompt_tokens":     100,
				"completion_tokens": 50,
				"total_tokens":      150,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func TestGenerate_Success(t *testing.T) {
	keywords := "sunset, ocean, golden hour, warm, peaceful"
	srv := fakeLLMServer(t, keywords, http.StatusOK)
	defer srv.Close()

	dir := t.TempDir()
	imgPath := makeTestJPEG(t, dir, "test.jpg", 100, 100)

	result, err := Generate(context.Background(), Request{
		ImagePath: imgPath,
		ApiURL:    srv.URL,
		ApiKey:    "test-key",
		Model:     "gpt-4o",
		Timeout:   10 * time.Second,
	})
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	if result != keywords {
		t.Errorf("Generate() = %q, want %q", result, keywords)
	}
}

func TestGenerate_MissingAPIKey(t *testing.T) {
	_, err := Generate(context.Background(), Request{
		ImagePath: "/tmp/nonexistent.jpg",
		ApiURL:    "http://localhost:9999",
		ApiKey:    "",
		Model:     "gpt-4o",
	})
	if err == nil || !strings.Contains(err.Error(), "API key") {
		t.Errorf("expected API key error, got: %v", err)
	}
}

func TestGenerate_MissingAPIURL(t *testing.T) {
	_, err := Generate(context.Background(), Request{
		ImagePath: "/tmp/nonexistent.jpg",
		ApiURL:    "",
		ApiKey:    "test-key",
		Model:     "gpt-4o",
	})
	if err == nil || !strings.Contains(err.Error(), "API URL") {
		t.Errorf("expected API URL error, got: %v", err)
	}
}

func TestGenerate_UnsupportedFormat(t *testing.T) {
	dir := t.TempDir()
	txtFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(txtFile, []byte("not an image"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Generate(context.Background(), Request{
		ImagePath: txtFile,
		ApiURL:    "http://localhost:9999",
		ApiKey:    "test-key",
		Model:     "gpt-4o",
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported image format") {
		t.Errorf("expected unsupported format error, got: %v", err)
	}
}

func TestGenerate_FileNotFound(t *testing.T) {
	_, err := Generate(context.Background(), Request{
		ImagePath: "/tmp/definitely-does-not-exist-12345.jpg",
		ApiURL:    "http://localhost:9999",
		ApiKey:    "test-key",
		Model:     "gpt-4o",
	})
	if err == nil || !strings.Contains(err.Error(), "preparing image") {
		t.Errorf("expected file error, got: %v", err)
	}
}

func TestParseKeywords_CommaSeparated(t *testing.T) {
	raw := "sunset, ocean, golden hour, warm, peaceful"
	got := ParseKeywords(raw)
	want := []string{"sunset", "ocean", "golden hour", "warm", "peaceful"}
	if len(got) != len(want) {
		t.Fatalf("ParseKeywords() returned %d keywords, want %d: %v", len(got), len(want), got)
	}
	for i, kw := range got {
		if kw != want[i] {
			t.Errorf("keyword[%d] = %q, want %q", i, kw, want[i])
		}
	}
}

func TestParseKeywords_NumberedList(t *testing.T) {
	raw := `1. sunset
2. ocean
3. golden hour
4. warm
5. peaceful`
	got := ParseKeywords(raw)
	if len(got) != 5 {
		t.Fatalf("ParseKeywords() returned %d keywords, want 5: %v", len(got), got)
	}
	if got[0] != "sunset" {
		t.Errorf("keyword[0] = %q, want %q", got[0], "sunset")
	}
}

func TestParseKeywords_BulletedList(t *testing.T) {
	raw := `- sunset
- ocean
- golden hour
- warm`
	got := ParseKeywords(raw)
	if len(got) != 4 {
		t.Fatalf("ParseKeywords() returned %d keywords, want 4: %v", len(got), got)
	}
	if got[2] != "golden hour" {
		t.Errorf("keyword[2] = %q, want %q", got[2], "golden hour")
	}
}

func TestParseKeywords_Dedup(t *testing.T) {
	raw := "sunset, ocean, sunset, warm, ocean"
	got := ParseKeywords(raw)
	if len(got) != 3 {
		t.Errorf("ParseKeywords() should deduplicate, got %d keywords: %v", len(got), got)
	}
}

func TestParseKeywords_MaxTen(t *testing.T) {
	parts := make([]string, 15)
	for i := range parts {
		parts[i] = strings.Repeat(string(rune('a'+i)), 3)
	}
	raw := strings.Join(parts, ", ")
	got := ParseKeywords(raw)
	if len(got) > maxKeywords {
		t.Errorf("ParseKeywords() returned %d keywords, want max %d", len(got), maxKeywords)
	}
}

func TestParseKeywords_Empty(t *testing.T) {
	got := ParseKeywords("")
	if len(got) != 0 {
		t.Errorf("ParseKeywords(\"\") returned %d keywords, want 0", len(got))
	}
}

func TestMimeForExt(t *testing.T) {
	tests := []struct {
		ext  string
		want string
		ok   bool
	}{
		{".jpg", "image/jpeg", true},
		{".jpeg", "image/jpeg", true},
		{".JPG", "image/jpeg", true},
		{".png", "image/png", true},
		{".gif", "image/gif", true},
		{".bmp", "image/bmp", true},
		{".tiff", "image/tiff", true},
		{".tif", "image/tiff", true},
		{".webp", "image/webp", true},
		{".txt", "", false},
		{".pdf", "", false},
		{"", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			got, ok := MimeForExt(tt.ext)
			if ok != tt.ok || got != tt.want {
				t.Errorf("MimeForExt(%q) = (%q, %v), want (%q, %v)", tt.ext, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestResizeImage_LargeImage(t *testing.T) {
	dir := t.TempDir()
	imgPath := makeTestJPEG(t, dir, "large.jpg", 1024, 768)

	data, err := resizeImage(imgPath)
	if err != nil {
		t.Fatalf("resizeImage() error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("resizeImage() returned empty bytes")
	}

	// Decode the result to check dimensions.
	img, err := jpeg.Decode(strings.NewReader(string(data)))
	if err != nil {
		t.Fatalf("decoding resized image: %v", err)
	}
	bounds := img.Bounds()
	if bounds.Dx() > maxPixels || bounds.Dy() > maxPixels {
		t.Errorf("resized image is %dx%d, expected max %dx%d", bounds.Dx(), bounds.Dy(), maxPixels, maxPixels)
	}
	// Check aspect ratio is approximately preserved (1024:768 = 4:3).
	if bounds.Dx() != 512 || bounds.Dy() != 384 {
		t.Errorf("resized image is %dx%d, expected 512x384", bounds.Dx(), bounds.Dy())
	}
}

func TestResizeImage_SmallImage(t *testing.T) {
	dir := t.TempDir()
	imgPath := makeTestJPEG(t, dir, "small.jpg", 100, 100)

	data, err := resizeImage(imgPath)
	if err != nil {
		t.Fatalf("resizeImage() error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("resizeImage() returned empty bytes")
	}

	// Should not resize — decode and check it's still 100x100.
	img, err := jpeg.Decode(strings.NewReader(string(data)))
	if err != nil {
		t.Fatalf("decoding image: %v", err)
	}
	bounds := img.Bounds()
	if bounds.Dx() != 100 || bounds.Dy() != 100 {
		t.Errorf("small image should not be resized, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}
