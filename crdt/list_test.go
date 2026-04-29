package crdt_test

import (
	"testing"

	. "6.5840-final-project/crdt"
)

func getListAsString(list YataList) string {
	panic("not implemented")
}

func TestBasic(t *testing.T) {
	list := MakeYataList(0)
	list.InsertItem(0, "Y")
	list.InsertItem(1, "A")
	list.InsertItem(2, "T")
	list.InsertItem(3, "A")
	
	if getListAsString(list) != "YATA" { t.FailNow() }
}

func TestMergeBasic(t *testing.T) {
	list0 := MakeYataList(0)
	list0.InsertItem(0, "Y")
	list0.InsertItem(1, "A")

	list1 := MakeYataList(1)
	list1.InsertItem(0, "T")
	list1.InsertItem(1, "A")

	update := list0.EncodeStateAsUpdate(nil)
	list1.ApplyUpdate(update)
	
	if getListAsString(list1) != "YATA" { t.FailNow() }
}
