package duplicates

import "github.com/cjbarker/rufus/internal/hasher"

// bkNode is a single node in the BK-tree.
type bkNode struct {
	idx      int            // index into the caller's images slice
	hash     uint64
	children map[int]*bkNode // keyed by Hamming distance from this node's hash
}

// bkTree is a metric tree that supports efficient nearest-neighbor search in
// Hamming space. For a query hash and threshold t it returns all indexed hashes
// within Hamming distance t in roughly O(log n) time per query, reducing
// pairwise duplicate detection from O(n²) to O(n log n) in practice.
//
// Multiple images with identical hashes are handled correctly: they are stored
// as a chain of children at distance 0 and are all returned by any search that
// matches their hash.
type bkTree struct {
	root *bkNode
}

// insert adds an image (identified by its index in the parent slice) with the
// given hash into the tree.
func (t *bkTree) insert(idx int, hash uint64) {
	node := &bkNode{idx: idx, hash: hash, children: make(map[int]*bkNode)}
	if t.root == nil {
		t.root = node
		return
	}
	cur := t.root
	for {
		d := hasher.HammingDistance(hash, cur.hash)
		if child, ok := cur.children[d]; ok {
			cur = child
		} else {
			cur.children[d] = node
			return
		}
	}
}

// search returns the indices of all nodes whose hash is within threshold
// Hamming distance of the query hash.
func (t *bkTree) search(hash uint64, threshold int) []int {
	if t.root == nil {
		return nil
	}
	var out []int
	t.searchNode(t.root, hash, threshold, &out)
	return out
}

func (t *bkTree) searchNode(n *bkNode, hash uint64, threshold int, out *[]int) {
	d := hasher.HammingDistance(hash, n.hash)
	if d <= threshold {
		*out = append(*out, n.idx)
	}
	// Only recurse into children whose distance from the current node places
	// them within the possible match range [d-threshold, d+threshold].
	lo := d - threshold
	if lo < 0 {
		lo = 0
	}
	hi := d + threshold
	for dist, child := range n.children {
		if dist >= lo && dist <= hi {
			t.searchNode(child, hash, threshold, out)
		}
	}
}
