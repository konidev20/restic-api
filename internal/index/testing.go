package index

import (
	"testing"

	"github.com/konidev20/restic-api/restic"
	"github.com/konidev20/restic-api/internal/test"
)

func TestMergeIndex(t testing.TB, mi *MasterIndex) ([]*Index, int) {
	finalIndexes := mi.finalizeNotFinalIndexes()
	for _, idx := range finalIndexes {
		test.OK(t, idx.SetID(restic.NewRandomID()))
	}

	test.OK(t, mi.MergeFinalIndexes())
	return finalIndexes, len(mi.idx)
}
