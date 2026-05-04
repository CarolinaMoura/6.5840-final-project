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

// Returns the advertise addresses of every registered signaling server,
// by reading all keys under the "signalings/" prefix.
func (ck *Clerk) Signalings() ([]string, rpc.Err) {
	ctx, cancel := context.WithTimeout(context.Background(), clerkTimeout)
	defer cancel()

	resp, err := ck.clnt.Get(ctx, "server/", clientv3.WithPrefix())
	if err != nil {
		fmt.Println("Error listing signalings:", err)
		return nil, rpc.ErrEtcd
	}
	addrs := make([]string, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		addrs = append(addrs, string(kv.Value))
	}
	return addrs, rpc.OK
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

// Registers (key, value) under a self-renewing lease. Returns a stop
// function that revokes the lease (and so deletes the key) when called.
// The lease also expires on its own if the process dies without calling stop.
func (ck *Clerk) RegisterWithLease(addr, value string, ttlSeconds int64) (func(), error) {
	ctx, cancel := context.WithCancel(context.Background())

	lease, err := ck.clnt.Grant(ctx, ttlSeconds)
	if err != nil {
		cancel()
		return nil, err
	}
	if _, err := ck.clnt.Put(ctx, "server/"+addr, value, clientv3.WithLease(lease.ID)); err != nil {
		cancel()
		return nil, err
	}
	ch, err := ck.clnt.KeepAlive(ctx, lease.ID)
	if err != nil {
		cancel()
		return nil, err
	}
	// Drain renewal acks so the channel buffer doesn't fill and stall keep-alive.
	go func() {
		for range ch {
		}
	}()

	return func() {
		cancel()
		revokeCtx, revokeCancel := context.WithTimeout(context.Background(), clerkTimeout)
		defer revokeCancel()
		ck.clnt.Revoke(revokeCtx, lease.ID)
	}, nil
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
