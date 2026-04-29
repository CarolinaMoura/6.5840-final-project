package crdt

type YataList struct {
	owner       ID
	items       []YataItem
	stateVector YataStateVector
}

// type update struct {
// 	item 	YataItem
// 	content any
// }

func (*YataList) GetStateVector() YataStateVector {
	panic("not implemented")
}

func (*YataList) ApplyUpdate(update []byte) {
	panic("not implemented")
}

func (*YataList) EncodeStateVector() []byte {
	panic("not implemented")
}

func (*YataList) EncodeStateAsUpdate(encodedTargetStateVector []byte) []byte {
	panic("not implemented")
}

func (*YataList) InsertItem(index int, content any) {
	panic("not implemented")
}

func (*YataList) DeleteItem(index int) {
	panic("not implemented")
}

func (*YataList) Length() {
	panic("not implemented")
}

func (*YataList) Items() []any {
	panic("not implemented")
}

func MakeYataList(uuid UUID) YataList {
	owner := ID{UserID: uuid, OpCounter: 0}
	return YataList{
		owner:       owner,
		items:       make([]YataItem, 2),
		stateVector: YataStateVector{owner},
	}
}
