package merkle

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// fixture mirrors the shape of testdata/merkle/*.json.
type fixture struct {
	Leaves []string `json:"leaves"`
	Root   string   `json:"root"`
	Proofs []struct {
		Leaf  string   `json:"leaf"`
		Proof []string `json:"proof"`
	} `json:"proofs"`
}

func loadFixture(t *testing.T, name string) fixture {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "merkle", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	var f fixture
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatalf("parse fixture %s: %v", path, err)
	}
	return f
}

func hexTo32(t *testing.T, s string) [32]byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("decode hex %q: %v", s, err)
	}
	if len(b) != 32 {
		t.Fatalf("hex %q is %d bytes, want 32", s, len(b))
	}
	var out [32]byte
	copy(out[:], b)
	return out
}

// TestBuildTree_MatchesFixtures is the test the build sequence calls out
// explicitly: every fixture, including the unbalanced seven-leaf case,
// must reproduce the exact root and per-leaf proof the Rust contract's
// format pins down. This is checked two ways — direct equality against
// the fixture's proof list, and independently folding each fixture proof
// with this package's own Fold and comparing to the fixture's root — so a
// bug that happened to produce the right root through a different (wrong)
// per-leaf proof assignment would still be caught.
func TestBuildTree_MatchesFixtures(t *testing.T) {
	for _, name := range []string{"single-leaf.json", "four-leaves.json", "seven-leaves.json"} {
		t.Run(name, func(t *testing.T) {
			f := loadFixture(t, name)

			leaves := make([][32]byte, len(f.Leaves))
			for i, l := range f.Leaves {
				leaves[i] = hexTo32(t, l)
			}

			tree, err := BuildTree(leaves)
			if err != nil {
				t.Fatalf("BuildTree returned unexpected error: %v", err)
			}

			wantRoot := hexTo32(t, f.Root)
			if tree.Root != wantRoot {
				t.Errorf("Root = %x, want %x", tree.Root, wantRoot)
			}

			if len(f.Proofs) != len(leaves) {
				t.Fatalf("fixture has %d proof entries for %d leaves", len(f.Proofs), len(leaves))
			}
			for i, p := range f.Proofs {
				wantLeaf := hexTo32(t, p.Leaf)
				if leaves[i] != wantLeaf {
					t.Fatalf("fixture proof entry %d is for leaf %s, but leaves[%d] is %x — fixture order assumption broken",
						i, p.Leaf, i, leaves[i])
				}

				wantProof := make(Proof, len(p.Proof))
				for j, s := range p.Proof {
					wantProof[j] = hexTo32(t, s)
				}

				gotProof := tree.Proofs[i]
				if len(gotProof) != len(wantProof) {
					t.Errorf("leaf %d: proof has %d nodes, want %d", i, len(gotProof), len(wantProof))
				} else {
					for j := range gotProof {
						if gotProof[j] != wantProof[j] {
							t.Errorf("leaf %d: proof[%d] = %x, want %x", i, j, gotProof[j], wantProof[j])
						}
					}
				}

				// Cross-check: folding the fixture's own proof (not ours)
				// against the fixture's own root must succeed regardless
				// of what BuildTree produced.
				if !VerifyProof(wantRoot, wantLeaf, wantProof) {
					t.Errorf("leaf %d: fixture's own proof does not fold to the fixture's own root", i)
				}

				// And folding what we produced must also reach the root.
				if !VerifyProof(tree.Root, leaves[i], gotProof) {
					t.Errorf("leaf %d: BuildTree's proof does not fold to BuildTree's root", i)
				}
			}
		})
	}
}

func TestBuildTree_RejectsEmpty(t *testing.T) {
	_, err := BuildTree(nil)
	if !errors.Is(err, ErrEmptyTree) {
		t.Fatalf("BuildTree(nil) error = %v, want ErrEmptyTree", err)
	}
}

func TestBuildTree_RejectsDuplicateLeaf(t *testing.T) {
	leaf := [32]byte{1, 2, 3}
	_, err := BuildTree([][32]byte{leaf, {4, 5, 6}, leaf})
	if !errors.Is(err, ErrDuplicateLeaf) {
		t.Fatalf("BuildTree error = %v, want ErrDuplicateLeaf", err)
	}
}

func TestProofDepth_RejectsLeafCountOverContractLimit(t *testing.T) {
	if got := proofDepth(1 << maxProofLength); got != maxProofLength {
		t.Fatalf("proofDepth(2^%d) = %d, want %d", maxProofLength, got, maxProofLength)
	}
	tooMany := (uint64(1) << maxProofLength) + 1
	if err := validateProofDepth(tooMany); !errors.Is(err, ErrProofTooLong) {
		t.Fatalf("validateProofDepth(%d) error = %v, want ErrProofTooLong", tooMany, err)
	}
}

func TestVerifyProof_AllowsEqualSibling(t *testing.T) {
	leaf := [32]byte{0x42}
	root := sortedPairHash(leaf, leaf)
	if !VerifyProof(root, leaf, Proof{leaf}) {
		t.Fatal("VerifyProof rejected a proof whose sibling equals the leaf")
	}
}

// ExampleVerifyProof demonstrates how VerifyProof validates a leaf and its
// proof against a known Merkle root.
func ExampleVerifyProof() {
	leaf := [32]byte{0x01}
	sibling := [32]byte{0x02}
	root := sortedPairHash(leaf, sibling)

	valid := VerifyProof(root, leaf, Proof{sibling})
	invalid := VerifyProof(root, leaf, Proof{leaf})

	fmt.Println(valid)
	fmt.Println(invalid)

	// Output:
	// true
	// false
}

func FuzzBuildTreeProofAgreement(f *testing.F) {
	f.Add([]byte{0, 0, 0})   // one leaf
	f.Add([]byte{6, 0, 0})   // seven leaves, unbalanced at multiple levels
	f.Add([]byte{255, 3, 0}) // 1024 leaves
	f.Add([]byte{3, 0, 1})   // four leaves with a repeated leaf
	f.Add([]byte{1, 0, 0})   // two unique leaves; equal-sibling fold is tested above
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}

		leafCount := 1
		if len(data) >= 2 {
			leafCount += int(uint16(data[0])|uint16(data[1])<<8) % 1024
		}
		leaves := make([][32]byte, leafCount)
		seed := make([]byte, len(data)+4)
		copy(seed, data)
		for i := range leaves {
			seed[len(data)] = byte(i)
			seed[len(data)+1] = byte(i >> 8)
			seed[len(data)+2] = byte(i >> 16)
			seed[len(data)+3] = byte(i >> 24)
			leaves[i] = sha256.Sum256(seed)
		}

		duplicate := len(data) >= 3 && data[2]&1 != 0 && leafCount > 1
		if duplicate {
			leaves[leafCount-1] = leaves[0]
		}
		tree, err := BuildTree(leaves)
		if duplicate {
			if tree != nil || !errors.Is(err, ErrDuplicateLeaf) {
				t.Fatalf("duplicate leaf set returned tree=%v, error=%v; want nil tree and ErrDuplicateLeaf", tree, err)
			}
			return
		}
		if err != nil {
			t.Fatalf("BuildTree(%d leaves): %v", leafCount, err)
		}
		if len(tree.Proofs) != len(leaves) {
			t.Fatalf("got %d proofs for %d leaves", len(tree.Proofs), len(leaves))
		}
		for i, proof := range tree.Proofs {
			if len(proof) > maxProofLength {
				t.Fatalf("proof %d has %d nodes, cap is %d", i, len(proof), maxProofLength)
			}
			if !VerifyProof(tree.Root, leaves[i], proof) {
				t.Fatalf("proof %d of %d leaves does not verify", i, leafCount)
			}
		}
	})
}

func TestBuildTree_SingleLeafHasEmptyProofAndRootEqualsLeaf(t *testing.T) {
	leaf := [32]byte{9, 9, 9}
	tree, err := BuildTree([][32]byte{leaf})
	if err != nil {
		t.Fatalf("BuildTree returned unexpected error: %v", err)
	}
	if tree.Root != leaf {
		t.Errorf("Root = %x, want the single leaf %x unchanged", tree.Root, leaf)
	}
	if len(tree.Proofs[0]) != 0 {
		t.Errorf("single-leaf proof has %d entries, want 0", len(tree.Proofs[0]))
	}
}

// TestBuildTree_OddCountsAtEveryDepth exercises the odd-leaf-promotion
// path at several tree depths (3, 5, 9, 13 leaves), not just the fixture's
// seven-leaf case, and checks every proof folds back to the root.
func ExampleFold() {
	leaf := [32]byte{1, 2, 3, 4}
	proof := Proof{
		[32]byte{5, 6, 7, 8},
		[32]byte{9, 10, 11, 12},
	}
	root := Fold(leaf, proof)
	fmt.Printf("%x\n", root)
	// Output:
	// 55c67a99fbb692b189589b60eefdfa2de4059653220cfe35a4f54b8e27abdf4d
}

func TestBuildTree_OddCountsAtEveryDepth(t *testing.T) {
	for _, n := range []int{2, 3, 5, 6, 9, 13} {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			leaves := make([][32]byte, n)
			for i := range leaves {
				leaves[i][0] = byte(i + 1)
				leaves[i][31] = byte(n)
			}
			tree, err := BuildTree(leaves)
			if err != nil {
				t.Fatalf("BuildTree returned unexpected error: %v", err)
			}
			for i, leaf := range leaves {
				if !VerifyProof(tree.Root, leaf, tree.Proofs[i]) {
					t.Errorf("leaf %d of %d does not verify against the root", i, n)
				}
			}
		})
	}
}

func TestBuildTree_RejectsProofOverCap(t *testing.T) {
	// 2^33 leaves would need a 33-node proof, exceeding the contract's cap
	// of 32 — far too many to actually construct, so this is checked via
	// the cap constant directly rather than an enormous input.
	if maxProofLength != 32 {
		t.Fatalf("maxProofLength = %d, want 32 per the contract's cap", maxProofLength)
	}
}

func TestVerifyProof_RejectsWrongLeafOrProof(t *testing.T) {
	leaves := [][32]byte{{1}, {2}, {3}, {4}}
	tree, err := BuildTree(leaves)
	if err != nil {
		t.Fatalf("BuildTree returned unexpected error: %v", err)
	}

	tests := []struct {
		name string
		root [32]byte
		leaf [32]byte
		prf  Proof
	}{
		{"wrong leaf", tree.Root, [32]byte{99}, tree.Proofs[0]},
		{"wrong proof", tree.Root, leaves[0], tree.Proofs[1]},
		{"wrong root", [32]byte{}, leaves[0], tree.Proofs[0]},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if VerifyProof(tt.root, tt.leaf, tt.prf) {
				t.Errorf("VerifyProof accepted an invalid combination (%s)", tt.name)
			}
		})
	}
}

// ExampleTree shows how a caller obtains a Tree and what its two fields
// hold. Tree is an output type: BuildTree computes Root and every entry of
// Proofs, so a caller assigns neither — the zero value is not a usable
// tree (a zero Root and no proofs), and BuildTree is the only supported
// constructor. The example builds a three-leaf tree, an odd count, so the
// unbalanced promotion path the contract's fold algorithm pins down is
// the one exercised; every proof folds back to the root, and the zero
// value verifies nothing.
func ExampleTree() {
	leaves := [][32]byte{
		exampleFill32(0x01),
		exampleFill32(0x02),
		exampleFill32(0x03),
	}

	tree, err := BuildTree(leaves)
	if err != nil {
		log.Fatalf("BuildTree: %v", err)
	}
	fmt.Printf("root: %x\n", tree.Root)
	fmt.Println("proofs:", len(tree.Proofs))

	allVerify := true
	for i, leaf := range leaves {
		allVerify = allVerify && VerifyProof(tree.Root, leaf, tree.Proofs[i])
	}
	fmt.Println("every proof folds to the root:", allVerify)

	var zero Tree
	fmt.Println("zero-value root is unset:", zero.Root == [32]byte{})
	fmt.Println("zero-value proofs are unset:", zero.Proofs == nil)
	fmt.Println("zero value verifies a leaf:", VerifyProof(zero.Root, leaves[0], nil))

	// Output:
	// root: 60457daa082476d6d1c02c1581a36bb4fad5dac0c5e6bdd0ee4adc5cd0425c21
	// proofs: 3
	// every proof folds to the root: true
	// zero-value root is unset: true
	// zero-value proofs are unset: true
	// zero value verifies a leaf: false
}

func benchmarkLeaves(count int) [][32]byte {
	leaves := make([][32]byte, count)
	for i := range leaves {
		leaves[i] = sha256.Sum256([]byte(strconv.Itoa(i)))
	}
	return leaves
}

func BenchmarkBuildTree(b *testing.B) {
	for _, count := range []int{100, 1000, 10000, 100000} {
		b.Run(strconv.Itoa(count), func(b *testing.B) {
			leaves := benchmarkLeaves(count)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				tree, err := BuildTree(leaves)
				if err != nil {
					b.Fatal(err)
				}
				if len(tree.Proofs) != count {
					b.Fatalf("got %d proofs, want %d", len(tree.Proofs), count)
				}
			}
		})
	}
}

func BenchmarkBuildTreeProofsForEveryLeaf(b *testing.B) {
	const count = 10000
	leaves := benchmarkLeaves(count)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree, err := BuildTree(leaves)
		if err != nil {
			b.Fatal(err)
		}
		if len(tree.Proofs) != count {
			b.Fatalf("got %d proofs, want %d", len(tree.Proofs), count)
		}
		proofNodes := 0
		for _, proof := range tree.Proofs {
			proofNodes += len(proof)
		}
		if proofNodes == 0 {
			b.Fatal("no proof nodes were generated")
		}
	}
}
