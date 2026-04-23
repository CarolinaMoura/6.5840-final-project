package list_test

import (
	"testing"
	. "ygo/list"
)

func TestNothing(t *testing.T) {
	list := MakeYataList(0)
	list.ApplyUpdate(nil)
}
