package crawler

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Supported image file extensions.
var imageExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".gif":  true,
	".bmp":  true,
	".tiff": true,
	".tif":  true,
	".webp": true,
}

// Result represents a discovered image file.
type Result struct {
	Path    string
	Size    int64
	ModTime time.Time
	Err     error
}

// Crawl walks the directory tree rooted at root and sends discovered image
// file paths to the returned channel. The channel is closed when crawling
// is complete.
func Crawl(root string, recursive bool) <-chan Result {
	ch := make(chan Result, 256)

	go func() {
		defer close(ch)

		if recursive {
			_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					ch <- Result{Path: path, Err: err}
					return nil // skip entry, keep walking
				}
				if d.IsDir() {
					return nil
				}
				if !isImageFile(d.Name()) {
					return nil
				}
				info, err := d.Info()
				if err != nil {
					ch <- Result{Path: path, Err: err}
					return nil
				}
				ch <- Result{Path: path, Size: info.Size(), ModTime: info.ModTime()}
				return nil
			})
		} else {
			entries, err := os.ReadDir(root)
			if err != nil {
				ch <- Result{Err: err}
				return
			}
			for _, entry := range entries {
				if entry.IsDir() || !isImageFile(entry.Name()) {
					continue
				}
				info, err := entry.Info()
				if err != nil {
					ch <- Result{Path: filepath.Join(root, entry.Name()), Err: err}
					continue
				}
				ch <- Result{
					Path:    filepath.Join(root, entry.Name()),
					Size:    info.Size(),
					ModTime: info.ModTime(),
				}
			}
		}
	}()

	return ch
}

func isImageFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return imageExtensions[ext]
}

// IsImageExtension checks if the given extension (with dot) is a supported image format.
func IsImageExtension(ext string) bool {
	return imageExtensions[strings.ToLower(ext)]
}

// SupportedExtensions returns a list of all supported image file extensions.
func SupportedExtensions() []string {
	exts := make([]string, 0, len(imageExtensions))
	for ext := range imageExtensions {
		exts = append(exts, ext)
	}
	return exts
}
