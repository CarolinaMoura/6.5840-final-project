package crdt_test

import (
	"testing"

	. "6.5840-final-project/crdt"
)

func TestNothing(t *testing.T) {
	list := MakeYataList(0)
	list.ApplyUpdate(nil)
}
