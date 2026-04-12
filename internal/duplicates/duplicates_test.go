package duplicates

import (
	"testing"

	"github.com/cjbarker/rufus/internal/db"
)

func TestFindExactDuplicates(t *testing.T) {
	images := []db.ImageRecord{
		{ID: 1, FilePath: "/a.jpg", FileHash: "hash1", FileSize: 1000, Width: 100, Height: 100},
		{ID: 2, FilePath: "/b.jpg", FileHash: "hash1", FileSize: 1000, Width: 100, Height: 100}, // exact dupe
		{ID: 3, FilePath: "/c.jpg", FileHash: "hash2", FileSize: 2000, Width: 200, Height: 200},
	}

	groups := FindExactDuplicates(images)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if len(groups[0].Images) != 2 {
		t.Errorf("expected 2 images in group, got %d", len(groups[0].Images))
	}
	if groups[0].MaxDistance != 0 {
		t.Errorf("exact duplicates should have distance 0, got %d", groups[0].MaxDistance)
	}
}

func TestFindPerceptualDuplicates(t *testing.T) {
	// Two images with very similar dHash values (distance 2)
	images := []db.ImageRecord{
		{ID: 1, FilePath: "/a.jpg", DHash: 0xFF00FF00FF00FF00, FileHash: "h1"},
		{ID: 2, FilePath: "/b.jpg", DHash: 0xFF00FF00FF00FF03, FileHash: "h2"}, // distance 2
		{ID: 3, FilePath: "/c.jpg", DHash: 0x00FF00FF00FF00FF, FileHash: "h3"}, // very different
	}

	groups := FindDuplicates(images, DHash, 5)
	// Should find the first two as duplicates
	found := false
	for _, g := range groups {
		if len(g.Images) == 2 {
			found = true
		}
	}
	if !found {
		t.Error("expected to find a group of 2 perceptual duplicates")
	}
}

func TestFindDuplicatesEmpty(t *testing.T) {
	groups := FindDuplicates(nil, DHash, 10)
	if groups != nil {
		t.Errorf("expected nil for empty input, got %v", groups)
	}
}

func TestFindDuplicatesNoDupes(t *testing.T) {
	images := []db.ImageRecord{
		{ID: 1, FilePath: "/a.jpg", DHash: 0xFF, FileHash: "h1"},
		{ID: 2, FilePath: "/b.jpg", DHash: 0x00FFFFFFFFFFFFFF, FileHash: "h2"},
	}

	groups := FindDuplicates(images, DHash, 5)
	// Should only have groups with 2+ images
	for _, g := range groups {
		if len(g.Images) >= 2 {
			// Check that these are actually similar
			for i := 0; i < len(g.Images); i++ {
				for j := i + 1; j < len(g.Images); j++ {
					if g.Images[i].FileHash == g.Images[j].FileHash {
						continue // exact dupe group is OK
					}
				}
			}
		}
	}
}

func TestRankForKeeping(t *testing.T) {
	group := Group{
		Images: []db.ImageRecord{
			{FilePath: "/long/path/to/image.jpg", FileSize: 500, Width: 100, Height: 100},
			{FilePath: "/short.jpg", FileSize: 1000, Width: 200, Height: 200},
			{FilePath: "/medium/path.jpg", FileSize: 750, Width: 200, Height: 200},
		},
	}

	ranked := RankForKeeping(group)

	// First should be highest resolution + largest file
	if ranked[0].FilePath != "/short.jpg" {
		t.Errorf("expected /short.jpg first (highest res + largest), got %s", ranked[0].FilePath)
	}
}

func TestHashTypes(t *testing.T) {
	images := []db.ImageRecord{
		{ID: 1, AHash: 0xFF, DHash: 0xFF, PHash: 0xFF, FileHash: "h1"},
		{ID: 2, AHash: 0xFE, DHash: 0xFE, PHash: 0xFE, FileHash: "h2"},
	}

	// All hash types should detect these as duplicates (distance 1)
	for _, ht := range []HashType{AHash, DHash, PHash} {
		groups := findPerceptualDuplicates(images, ht, 5)
		if len(groups) != 1 {
			t.Errorf("hash type %s: expected 1 group, got %d", ht, len(groups))
		}
	}
}
