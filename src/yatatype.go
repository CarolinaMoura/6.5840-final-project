package types

type UUID uint64

type ID struct {
	UserID UUID
	OpCounter uint32
}

type YataItem struct {
	id          ID
    left        ID
    right       ID
    originLeft  ID
    originRight ID
    deleted     bool
	content 	any
}

type YataStateVector []ID

type YataType interface {
	GetStateVector() YataStateVector
	ApplyUpdate(update []byte)
	EncodeStateVector() []byte
	EncodeStateAsUpdate(encodedTargetStateVector []byte)
}

