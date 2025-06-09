package proof

import (
	"bytes"
	"crypto/sha256"
	"math/bits"

	proof_pb "github.com/celestiaorg/celestia-node/share/proof/pb"
)

// Hash prefixes for Tendermint-style hashing to prevent second pre-image attacks
var (
	leafPrefix  = []byte{0x00} // Prefix for leaf node hashes
	innerPrefix = []byte{0x01} // Prefix for inner node hashes
)

// MerkleProof represents a Merkle inclusion proof for a range of items [Start:End)
type MerkleProof struct {
	Total        int64    `json:"total"`         // Total number of items in the original tree
	Start        int64    `json:"start"`         // Start index of the range being proven (inclusive)
	End          int64    `json:"end"`           // End index of the range being proven (exclusive)
	SubtreeRoots [][]byte `json:"subtree_roots"` // Hashes of subtrees needed to reconstruct the full tree
}

// NewProof creates a Merkle inclusion proof for items[start:end)
func NewProof(items [][]byte, start, end int64) *MerkleProof {
	total := int64(len(items))
	// Build the proof by systematically traversing the tree and collecting
	// subtree roots that are needed to reconstruct the full tree
	subtreeRoots, err := buildRangeProof(items, start, end)
	if err != nil {
		panic(err)
	}

	return &MerkleProof{
		Total:        total,
		Start:        start,
		End:          end,
		SubtreeRoots: subtreeRoots,
	}
}

// buildRangeProof creates a proof by systematically traversing the Merkle tree:
//
// 1. It recursively traverses the tree following the same structure as the original tree
// 2. At each node, it decides whether to include the node's hash in the proof or recurse deeper
// 3. A subtree hash is included when the subtree has no overlap with the proven range
// 4. The function recurses deeper when the subtree overlaps with the proven range
//
// The 'includeNode' parameter controls whether the current subtree should be added
// to the proof. This creates a systematic way to collect exactly the subtree roots
// needed for verification.
func buildRangeProof(items [][]byte, proofStart, proofEnd int64) ([][]byte, error) {
	proof := [][]byte{}
	treeSize := int64(len(items))

	// Convert all items to leaf hashes upfront
	// This represents the leaf level of the Merkle tree
	leafHashes := make([][]byte, len(items))
	for i, item := range items {
		leafHashes[i] = leafHash(item)
	}

	// Recursive function that traverses the tree and builds the proof
	// start, end: the range of leaves this subtree covers
	// includeNode: whether this subtree's hash should be added to the proof
	var recurse func(start, end int64, includeNode bool) ([]byte, error)

	recurse = func(start, end int64, includeNode bool) ([]byte, error) {
		// Handle case where we've gone beyond the actual tree size
		if start >= treeSize {
			return nil, nil
		}

		// Base case: we've reached a single leaf
		if end-start == 1 {
			leafHash := leafHashes[start]

			// If this leaf is outside the proven range AND we need it for the proof,
			// add it to the proof collection
			if (start < proofStart || start >= proofEnd) && includeNode {
				proof = append(proof, leafHash)
			}

			// Always return the leaf hash for tree construction
			return leafHash, nil
		}

		// Determine whether the subtrees of this node need to be included in the proof
		newIncludeNode := includeNode

		// Key insight: If this entire subtree has no overlap with the proven range,
		// we don't need to include its individual subtrees - we can represent
		// the entire subtree with a single hash
		if (end <= proofStart || start >= proofEnd) && includeNode {
			newIncludeNode = false // Stop recursing, use this subtree's hash instead
		}

		// Split the range using the same logic as the original tree construction
		// This ensures we follow the exact same tree structure
		k := getSplitPoint(end - start)

		// Recursively process left subtree [start, start+k)
		left, err := recurse(start, start+k, newIncludeNode)
		if err != nil {
			return nil, err
		}

		// Recursively process right subtree [start+k, end)
		right, err := recurse(start+k, end, newIncludeNode)
		if err != nil {
			return nil, err
		}

		// Combine left and right subtrees to get this subtree's hash
		var hash []byte
		if right == nil {
			// Handle case where right subtree doesn't exist (tree not full)
			hash = left
		} else {
			hash = innerHash(left, right)
		}

		// Critical decision point: If we need this subtree's hash for the proof
		// but we're not recursing into its children (newIncludeNode == false),
		// then add this subtree's hash to the proof
		if includeNode && !newIncludeNode {
			proof = append(proof, hash)
		}

		return hash, nil
	}

	// Calculate the size of the full tree (next power of 2)
	// This ensures we traverse the complete tree structure
	fullTreeSize := getSplitPoint(treeSize) * 2
	if fullTreeSize < 1 {
		fullTreeSize = 1
	}

	// Start the recursive traversal from the root
	_, err := recurse(0, fullTreeSize, true)
	if err != nil {
		return nil, err
	}

	return proof, nil
}

// Verify checks if the proof is valid for the given root hash and items:
//
// 1. Validating that the provided items match the stored leaf hashes
// 2. Reconstructing the tree root using the proven leaves and stored subtree roots
// 3. Comparing the reconstructed root with the expected root
func (p *MerkleProof) Verify(rootHash []byte, items [][]byte) bool {
	// Basic structural validation
	if int64(len(items)) != p.End-p.Start {
		return false
	}

	leafHashes := make([][]byte, p.End-p.Start)
	// Verify that the provided items hash to the stored leaf hashes
	// This ensures the user is providing the correct data
	for i, item := range items {
		leafHashes[i] = leafHash(item)
	}

	// Reconstruct the tree root using the two-phase approach
	computedRoot := p.computeRoot(leafHashes)

	// Verify the reconstructed root matches the expected root
	return bytes.Equal(rootHash, computedRoot)
}

// computeRoot reconstructs the Merkle tree root using a two-phase approach
//
// Phase 1: Reconstruct a subtree that contains the entire proven range
// Phase 2: Combine the result with any remaining subtree roots to get the full tree root
func (p *MerkleProof) computeRoot(leafHashes [][]byte) []byte {
	if p.Total == 0 {
		return emptyHash()
	}
	if p.Total == 1 {
		return leafHashes[0]
	}

	subtreeRoots := make([][]byte, len(p.SubtreeRoots))
	copy(subtreeRoots, p.SubtreeRoots)

	// Phase 1: Calculate the minimum subtree size that contains the entire proven range
	// This is the smallest power-of-2 subtree that starts at index 0 and contains
	// all items up to (but not including) p.End
	proofRangeSubtreeEstimate := max(getSplitPoint(p.End)*2, 1)

	// Recursive function to reconstruct the tree following the same structure
	// as the original tree and the proof generation
	var computeRoot func(start, end int64) []byte
	computeRoot = func(start, end int64) []byte {
		// Base case: single leaf
		if end-start == 1 {
			if start >= p.Start && start < p.End {
				// This leaf is in our proven range - use the provided leaf hash
				if len(leafHashes) > 0 {
					leaf := leafHashes[0]
					leafHashes = leafHashes[1:] // pop from front
					return leaf
				}
			} else {
				// This leaf is outside our proven range - use a stored subtree root
				if len(subtreeRoots) > 0 {
					root := subtreeRoots[0]
					subtreeRoots = subtreeRoots[1:] // pop from front
					return root
				}
			}
			return emptyHash()
		}

		// If this entire range is outside the proven range, use a stored subtree root
		if end <= p.Start || start >= p.End {
			if len(subtreeRoots) > 0 {
				root := subtreeRoots[0]
				subtreeRoots = subtreeRoots[1:] // pop from front
				return root
			}
			return emptyHash()
		}

		// This range overlaps with the proven range - split and recurse
		// Use the same split logic as the original tree
		k := getSplitPoint(end - start)
		left := computeRoot(start, start+k)
		right := computeRoot(start+k, end)

		// Handle case where right subtree doesn't exist
		if len(right) == 0 {
			return left
		}

		return innerHash(left, right)
	}

	// Phase 1: Reconstruct the subtree containing the proven range
	rootHash := computeRoot(0, proofRangeSubtreeEstimate)

	// Phase 2: Combine with any remaining subtree roots
	// These represent parts of the tree that are outside the subtree we just reconstructed
	for len(subtreeRoots) > 0 {
		node := subtreeRoots[0]
		subtreeRoots = subtreeRoots[1:] // pop from front
		rootHash = innerHash(rootHash, node)
	}

	return rootHash
}

func (p *MerkleProof) ToProto() *proof_pb.MerkleProof {
	if p == nil {
		return nil
	}
	return &proof_pb.MerkleProof{
		Total:        p.Total,
		Start:        p.Start,
		End:          p.End,
		SubtreeRoots: p.SubtreeRoots,
	}
}

func MerkleProofFromProto(p *proof_pb.MerkleProof) *MerkleProof {
	if p == nil {
		return nil
	}
	return &MerkleProof{
		Total:        p.Total,
		Start:        p.Start,
		End:          p.End,
		SubtreeRoots: p.SubtreeRoots,
	}
}

// getSplitPoint calculates where to split a range when building a balanced binary tree
//
// For a range of length n, this returns the largest power of 2 that is less than n.
// If n is already a power of 2, it returns n/2.
//
// 1. The left subtree is always a perfect binary tree (power of 2 size)
// 2. The right subtree contains the remaining elements
// 3. The tree is as balanced as possible
func getSplitPoint(length int64) int64 {
	uLength := uint(length)
	bitlen := bits.Len(uLength)
	k := int64(1 << uint(bitlen-1))
	if k == length {
		k >>= 1
	}
	return k
}

// leafHash computes the hash of a leaf node
// Uses the leaf prefix to prevent second pre-image attacks
func leafHash(data []byte) []byte {
	h := sha256.Sum256(append(leafPrefix, data...))
	return h[:]
}

// innerHash computes the hash of an inner node from its left and right children
// Uses the inner prefix to prevent second pre-image attacks
func innerHash(left, right []byte) []byte {
	h := sha256.Sum256(append(append(innerPrefix, left...), right...))
	return h[:]
}

// emptyHash returns the hash used for empty/non-existent nodes
func emptyHash() []byte {
	h := sha256.Sum256([]byte{})
	return h[:]
}
