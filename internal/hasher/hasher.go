package hasher

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math/bits"
	"os"

	"github.com/corona10/goimagehash"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

// HashResult contains all computed hashes for an image.
type HashResult struct {
	FilePath string
	FileHash string // SHA-256 hex of file content
	AHash    uint64
	DHash    uint64
	PHash    uint64
	Width    int
	Height   int
	Format   string
}

// HashFile computes the SHA-256 file hash and perceptual hashes for an image.
func HashFile(path string) (*HashResult, error) {
	fileHash, err := sha256File(path)
	if err != nil {
		return nil, fmt.Errorf("computing file hash: %w", err)
	}

	img, format, err := decodeImage(path)
	if err != nil {
		return nil, fmt.Errorf("decoding image: %w", err)
	}

	bounds := img.Bounds()
	result := &HashResult{
		FilePath: path,
		FileHash: fileHash,
		Width:    bounds.Dx(),
		Height:   bounds.Dy(),
		Format:   format,
	}

	aHash, err := goimagehash.AverageHash(img)
	if err != nil {
		return nil, fmt.Errorf("computing ahash: %w", err)
	}
	result.AHash = aHash.GetHash()

	dHash, err := goimagehash.DifferenceHash(img)
	if err != nil {
		return nil, fmt.Errorf("computing dhash: %w", err)
	}
	result.DHash = dHash.GetHash()

	pHash, err := goimagehash.PerceptionHash(img)
	if err != nil {
		return nil, fmt.Errorf("computing phash: %w", err)
	}
	result.PHash = pHash.GetHash()

	return result, nil
}

// HammingDistance computes the Hamming distance between two hash values.
func HammingDistance(a, b uint64) int {
	return bits.OnesCount64(a ^ b)
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func decodeImage(path string) (image.Image, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = f.Close() }()

	img, format, err := image.Decode(f)
	if err != nil {
		return nil, "", err
	}
	return img, format, nil
}
