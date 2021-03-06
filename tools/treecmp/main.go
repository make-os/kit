package main

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/cosmos/iavl"
	"github.com/fatih/color"
	"github.com/google/go-cmp/cmp"
	"github.com/k0kubun/pp"
	"github.com/make-os/kit/logic/keepers"
	"github.com/make-os/kit/storage"
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/util"
	fmt2 "github.com/make-os/kit/util/colorfmt"
	"github.com/make-os/kit/util/crypto"
	tmdb "github.com/tendermint/tm-db"
)

func getAdapter(stateDBPath string) tmdb.DB {
	stateTreeDB, err := storage.NewBadgerTMDB("")
	if err != nil {
		panic(err)
	}
	return stateTreeDB
}

type Diffs struct {
	k         []byte
	pairs     [][]byte
	pairsPath []string
}

func cmpTree(t *TreePath, with *TreePath) []Diffs {
	var res []Diffs
	t.tree.Iterate(func(key, val []byte) bool {
		_, withVal := with.tree.Get(key)
		if !bytes.Equal(val, withVal) {
			res = append(res, Diffs{k: key, pairs: [][]byte{val, withVal}, pairsPath: []string{t.path, with.path}})
		}
		return false
	})
	return res
}

type TreePath struct {
	tree *iavl.MutableTree
	path string
}

func cmpIndexKey(pathA, pathB string) string {
	var strs []string
	strs = append(strs, strings.Split(pathA, "")...)
	strs = append(strs, strings.Split(pathB, "")...)
	sort.Strings(strs)
	return crypto.Hash20Hex([]byte(strings.Join(strs, "")))
}

func findAndPrintDiffKeys(version int64, paths ...string) []Diffs {
	var trees []*TreePath

	// Load trees
	for _, p := range paths {
		adapter := getAdapter(p)
		tree, err := iavl.NewMutableTree(adapter, 5000)
		if err != nil {
			panic(err)
		}
		tree.Load()
		tree.LoadVersion(version)
		trees = append(trees, &TreePath{tree: tree, path: p})
		adapter.Close()
	}

	var result []Diffs
	cmpIndex := map[string]struct{}{}

	for _, tree := range trees {
		for _, withTree := range trees {
			idxKey := cmpIndexKey(tree.path, withTree.path)
			if _, ok := cmpIndex[idxKey]; ok {
				continue
			}
			if tree != withTree {
				cmpRes := cmpTree(tree, withTree)
				for _, res := range cmpRes {
					result = append(result, res)
				}
			}
			cmpIndex[idxKey] = struct{}{}
		}
	}

	return result
}

func printBytesDiff(diffs []Diffs) {
	for i, diff := range diffs {
		fmt.Printf("Diff (%d): %s vs %s\n", i, fmt2.GreenString(diff.pairsPath[0]), fmt2.RedString(diff.pairsPath[1]))
		fmt.Println(cmp.Diff(diff.pairs[0], diff.pairs[1]))
	}
}

func printRawStrDiff(diffs []Diffs) {
	for i, diff := range diffs {
		fmt.Printf("Diff (%d): %s vs %s\n", i, fmt2.GreenString(diff.pairsPath[0]), color.RedString(diff.pairsPath[1]))
		pp.Println(string(diff.pairs[0]))
		fmt.Print("\n")
		pp.Println(string(diff.pairs[1]))
	}
}

func main() {
	diffs := findAndPrintDiffKeys(
		1505,
		"/Users/ncodes/.kit_dev_node1/1/data/appstate.db",
		"/Users/ncodes/.kit_dev_node2/1/data/appstate.db")

	// printRawStrDiff(diffs)
	printBytesDiff(diffs)

	// Print specific objects
	for _, diff := range diffs {
		if string(diff.k[:2]) == (keepers.TagRepo + ":") {
			var r, r2 state.Repository
			util.ToObject(diff.pairs[0], &r)
			util.ToObject(diff.pairs[1], &r2)
			pp.Println(r)
			pp.Println(r2)
		}
	}
}
