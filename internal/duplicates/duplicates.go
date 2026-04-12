package duplicates

import (
	"sort"

	"github.com/cjbarker/rufus/internal/db"
	"github.com/cjbarker/rufus/internal/hasher"
)

// HashType specifies which perceptual hash to use for comparison.
type HashType string

const (
	AHash HashType = "ahash"
	DHash HashType = "dhash"
	PHash HashType = "phash"
	Exact HashType = "exact"
)

// Group represents a set of duplicate images.
type Group struct {
	Images      []db.ImageRecord
	MaxDistance  int
	HashType    HashType
}

// FindDuplicates finds groups of duplicate images based on perceptual hash similarity.
func FindDuplicates(images []db.ImageRecord, hashType HashType, threshold int) []Group {
	if len(images) == 0 {
		return nil
	}

	// Find exact duplicates by file hash first
	exactGroups := findExactDuplicates(images)

	// Find perceptual duplicates
	perceptualGroups := findPerceptualDuplicates(images, hashType, threshold)

	// Merge groups, preferring perceptual groups that may encompass exact dupes
	return mergeGroups(exactGroups, perceptualGroups)
}

// FindExactDuplicates groups images by identical SHA-256 file hash.
func FindExactDuplicates(images []db.ImageRecord) []Group {
	return findExactDuplicates(images)
}

func findExactDuplicates(images []db.ImageRecord) []Group {
	hashMap := make(map[string][]db.ImageRecord)
	for _, img := range images {
		hashMap[img.FileHash] = append(hashMap[img.FileHash], img)
	}

	var groups []Group
	for _, imgs := range hashMap {
		if len(imgs) < 2 {
			continue
		}
		groups = append(groups, Group{
			Images:   imgs,
			MaxDistance: 0,
			HashType: "exact",
		})
	}

	sortGroups(groups)
	return groups
}

func findPerceptualDuplicates(images []db.ImageRecord, hashType HashType, threshold int) []Group {
	n := len(images)

	// Union-Find for grouping
	parent := make([]int, n)
	for i := range parent {
		parent[i] = i
	}

	find := func(x int) int {
		for parent[x] != x {
			parent[x] = parent[parent[x]]
			x = parent[x]
		}
		return x
	}

	union := func(x, y int) {
		px, py := find(x), find(y)
		if px != py {
			parent[px] = py
		}
	}

	// Compare all pairs — for large datasets a BK-tree would be better,
	// but this O(n^2) approach works well for typical photo libraries (< 100K images).
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			dist := distance(images[i], images[j], hashType)
			if dist <= threshold {
				union(i, j)
			}
		}
	}

	// Collect groups
	groupMap := make(map[int][]int)
	for i := 0; i < n; i++ {
		root := find(i)
		groupMap[root] = append(groupMap[root], i)
	}

	var groups []Group
	for _, indices := range groupMap {
		if len(indices) < 2 {
			continue
		}

		imgs := make([]db.ImageRecord, len(indices))
		maxDist := 0
		for k, idx := range indices {
			imgs[k] = images[idx]
		}

		// Compute max pairwise distance within group
		for k := 0; k < len(indices); k++ {
			for l := k + 1; l < len(indices); l++ {
				d := distance(images[indices[k]], images[indices[l]], hashType)
				if d > maxDist {
					maxDist = d
				}
			}
		}

		groups = append(groups, Group{
			Images:     imgs,
			MaxDistance: maxDist,
			HashType:   hashType,
		})
	}

	sortGroups(groups)
	return groups
}

func distance(a, b db.ImageRecord, hashType HashType) int {
	switch hashType {
	case AHash:
		return hasher.HammingDistance(a.AHash, b.AHash)
	case DHash:
		return hasher.HammingDistance(a.DHash, b.DHash)
	case PHash:
		return hasher.HammingDistance(a.PHash, b.PHash)
	default:
		return hasher.HammingDistance(a.DHash, b.DHash)
	}
}

func mergeGroups(exact, perceptual []Group) []Group {
	// Build a set of image IDs that appear in any exact-duplicate group.
	// Perceptual groups that are fully covered by a single exact group are
	// redundant (same images, no new information) and are dropped to avoid
	// showing the same files twice in the output.
	exactIDs := make(map[int64]bool)
	for _, g := range exact {
		for _, img := range g.Images {
			exactIDs[img.ID] = true
		}
	}

	all := make([]Group, 0, len(exact)+len(perceptual))
	all = append(all, exact...)
	for _, g := range perceptual {
		// Count how many members are not already in an exact group.
		var novel int
		for _, img := range g.Images {
			if !exactIDs[img.ID] {
				novel++
			}
		}
		// Keep the perceptual group only if it adds members beyond exact groups.
		// A group with novel == 0 is a pure subset of exact duplicates; skip it.
		// A group with novel >= 1 contains at least one near-duplicate not caught
		// by exact matching, so it's worth surfacing.
		if novel >= 1 {
			all = append(all, g)
		}
	}
	return all
}

func sortGroups(groups []Group) {
	sort.Slice(groups, func(i, j int) bool {
		return len(groups[i].Images) > len(groups[j].Images)
	})
}

// RankForKeeping returns the images in a group sorted by "keep priority" (best first).
// Higher resolution, larger file size, and shorter path are preferred.
func RankForKeeping(group Group) []db.ImageRecord {
	ranked := make([]db.ImageRecord, len(group.Images))
	copy(ranked, group.Images)

	sort.Slice(ranked, func(i, j int) bool {
		// Prefer higher resolution
		pixelsI := ranked[i].Width * ranked[i].Height
		pixelsJ := ranked[j].Width * ranked[j].Height
		if pixelsI != pixelsJ {
			return pixelsI > pixelsJ
		}
		// Prefer larger file (less compressed)
		if ranked[i].FileSize != ranked[j].FileSize {
			return ranked[i].FileSize > ranked[j].FileSize
		}
		// Prefer shorter path
		return len(ranked[i].FilePath) < len(ranked[j].FilePath)
	})

	return ranked
}
