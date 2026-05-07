package kv

import (
	"bytes"
	"context"
	"sync"

	"6.5840-final-project/encoder"
	"6.5840-final-project/kv/kvpb"
	"6.5840-final-project/rsm"
	"6.5840-final-project/rsm/rpc"
)

type VersionedValue struct {
	Value   string
	Version rpc.Tversion
}

type CommandType int

const (
	Get CommandType = iota
	Put
)

// Req is what we push through Raft. It lives inside rsm.Op.Req (which is
// `any`), so the encoder needs Req registered to round-trip it.
type Req struct {
	Type    CommandType
	GetArgs rpc.GetArgs
	PutArgs rpc.PutArgs
}

func init() {
	encoder.Register(rsm.Op{})
	encoder.Register(Req{})
}

type KVServer struct {
	// Embedded so KVServer satisfies the kvpb.KVServer interface and can be
	// registered directly on a gRPC server. The Get/Put methods below
	// override the unimplemented stubs.
	kvpb.UnimplementedKVServer

	me  int
	rsm *rsm.RSM

	mu    sync.Mutex
	store map[string]VersionedValue
}

// Caller has to call Bind so the KV can submit ops back through RSM
func NewKVServer(me int) *KVServer {
	return &KVServer{
		me:    me,
		store: make(map[string]VersionedValue),
	}
}

func (kv *KVServer) Bind(r *rsm.RSM) {
	kv.rsm = r
}

func (kv *KVServer) get(args rpc.GetArgs, reply *rpc.GetReply) {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	vv, ok := kv.store[args.Key]
	if !ok {
		reply.Err = rpc.ErrNoKey
		return
	}
	reply.Value = vv.Value
	reply.Version = vv.Version
	reply.Err = rpc.OK
}

func (kv *KVServer) put(args rpc.PutArgs, reply *rpc.PutReply) {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	vv, ok := kv.store[args.Key]
	if !ok && args.Version != 0 {
		reply.Err = rpc.ErrNoKey
		return
	}
	if ok && vv.Version != args.Version {
		reply.Err = rpc.ErrVersion
		return
	}
	kv.store[args.Key] = VersionedValue{Value: args.Value, Version: args.Version + 1}
	reply.Err = rpc.OK
}

func (kv *KVServer) DoOp(req any) any {
	op, ok := req.(Req)
	if !ok {
		panic("kv: DoOp got non-Req")
	}

	switch op.Type {
	case Get:
		reply := rpc.GetReply{}
		kv.get(op.GetArgs, &reply)
		return reply
	case Put:
		reply := rpc.PutReply{}
		kv.put(op.PutArgs, &reply)
		return reply
	}
	panic("kv: unknown CommandType")
}

func (kv *KVServer) Snapshot() []byte {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	w := new(bytes.Buffer)
	e := encoder.NewEncoder(w)
	if err := e.Encode(kv.store); err != nil {
		panic(err)
	}
	return w.Bytes()
}

func (kv *KVServer) Restore(data []byte) {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	r := bytes.NewBuffer(data)
	d := encoder.NewDecoder(r)

	var store map[string]VersionedValue
	if err := d.Decode(&store); err != nil {
		panic(err)
	}
	kv.store = store
}

// Get is the gRPC handler. Submits a Get op to the RSM and translates the
// reply into proto form. Returns ErrWrongLeader (as a string) when this
// node isn't the current raft leader so the clerk can retry against a
// different peer.
func (kv *KVServer) Get(ctx context.Context, in *kvpb.GetArgs) (*kvpb.GetReply, error) {
	args := rpc.GetArgs{Key: in.Key}
	err, res := kv.rsm.Submit(Req{Type: Get, GetArgs: args})
	if err == rpc.ErrWrongLeader {
		return &kvpb.GetReply{Err: string(rpc.ErrWrongLeader)}, nil
	}
	reply := res.(rpc.GetReply)
	return &kvpb.GetReply{
		Value:   reply.Value,
		Version: uint64(reply.Version),
		Err:     string(reply.Err),
	}, nil
}

// Put is the gRPC handler. Same shape as Get.
func (kv *KVServer) Put(ctx context.Context, in *kvpb.PutArgs) (*kvpb.PutReply, error) {
	args := rpc.PutArgs{
		Key:     in.Key,
		Value:   in.Value,
		Version: rpc.Tversion(in.Version),
	}
	err, res := kv.rsm.Submit(Req{Type: Put, PutArgs: args})
	if err == rpc.ErrWrongLeader {
		return &kvpb.PutReply{Err: string(rpc.ErrWrongLeader)}, nil
	}
	reply := res.(rpc.PutReply)
	return &kvpb.PutReply{Err: string(reply.Err)}, nil
}
