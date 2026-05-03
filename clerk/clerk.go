package clerk

import (
	"context"
	"fmt"
	"time"

	rpc "6.5840-final-project/rsm/rpc"
	clientv3 "go.etcd.io/etcd/client/v3"
)

const clerkTimeout = 5 * time.Second
const dialTimeout = 5 * time.Second

type Clerk struct {
	clnt *clientv3.Client
}

func MakeClerk(servers []string) (*Clerk, error) {
	cfg := clientv3.Config{
		Endpoints:   servers,
		DialTimeout: dialTimeout,
	}
	clnt, err := clientv3.New(cfg)
	if err != nil {
		return nil, err
	}
	return &Clerk{clnt: clnt}, nil
}

func (ck *Clerk) Close() {
	ck.clnt.Close()
}

func (ck *Clerk) Get(key string) (string, rpc.Tversion, rpc.Err) {
	ctx, cancel := context.WithTimeout(context.Background(), clerkTimeout)
	defer cancel()

	resp, err := ck.clnt.Get(ctx, key)
	if err != nil {
		fmt.Println("Error getting key:", err)
		return "", 0, rpc.ErrEtcd
	}
	if len(resp.Kvs) == 0 {
		return "", 0, rpc.ErrNoKey
	}
	kv := resp.Kvs[0]
	return string(kv.Value), rpc.Tversion(kv.Version), rpc.OK
}

func (ck *Clerk) Put(key, value string, version rpc.Tversion) rpc.Err {
	ctx, cancel := context.WithTimeout(context.Background(), clerkTimeout)
	defer cancel()

	// Put only if the key's current version matches.
	// On failure, the Else-Get tells us whether the key was missing
	// (ErrNoKey) or merely at a different version (ErrVersion).
	resp, err := ck.clnt.Txn(ctx).
		If(clientv3.Compare(clientv3.Version(key), "=", int64(version))).
		Then(clientv3.OpPut(key, value)).
		Else(clientv3.OpGet(key)).
		Commit()
	if err != nil {
		fmt.Println("Error putting key:", err)
		return rpc.ErrEtcd
	}
	if resp.Succeeded {
		return rpc.OK
	}
	if getResp := resp.Responses[0].GetResponseRange(); getResp == nil || len(getResp.Kvs) == 0 {
		return rpc.ErrNoKey
	}
	return rpc.ErrVersion
}

// [TODO] implement Sync
