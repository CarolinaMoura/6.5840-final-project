package clerk

import (
	"context"
	"fmt"
	"time"

	"6.5840-final-project/kv/kvpb"
	rpc "6.5840-final-project/rsm/rpc"

	"google.golang.org/grpc"
)

const callTimeout = 5 * time.Second

type Clerk struct {
	peers   []kvpb.KVClient // gRPC stubs, one per kv peer
	servers []string        // advertise addresses returned by Signalings()
	leader  int             // last successful leader (index into peers[])
}

// conns is one gRPC client connection per peer (shared with the raft layer
// — kv stubs are built on top of the same connections to avoid opening a
// second TCP/H2 link per peer). servers is the advertise-address list
// returned by Signalings(). The caller owns conns and is responsible for
// closing them on shutdown.
func MakeClerk(conns []*grpc.ClientConn, servers []string) (*Clerk, error) {
	if len(conns) == 0 {
		return nil, fmt.Errorf("MakeClerk: conns is empty")
	}
	peers := make([]kvpb.KVClient, len(conns))
	for i, c := range conns {
		peers[i] = kvpb.NewKVClient(c)
	}
	return &Clerk{peers: peers, servers: servers}, nil
}

func (ck *Clerk) Leader() int {
	return ck.leader
}

func (ck *Clerk) Get(key string) (string, rpc.Tversion, rpc.Err) {
	args := &kvpb.GetArgs{Key: key}

	leader := ck.leader
	for {
		ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
		reply, err := ck.peers[leader].Get(ctx, args)
		cancel()
		ok := err == nil
		if ok {
			if rpc.Err(reply.Err) == rpc.ErrWrongLeader {
				leader = (leader + 1) % len(ck.peers)
			} else {
				ck.leader = leader
				return reply.Value, rpc.Tversion(reply.Version), rpc.Err(reply.Err)
			}
		} else {
			leader = (leader + 1) % len(ck.peers)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (ck *Clerk) Put(key, value string, version rpc.Tversion) rpc.Err {
	args := &kvpb.PutArgs{Key: key, Value: value, Version: uint64(version)}

	leader := ck.leader

	for {
		ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
		firstReply, err := ck.peers[leader].Put(ctx, args)
		cancel()
		firstOk := err == nil
		if firstOk {
			if rpc.Err(firstReply.Err) == rpc.ErrWrongLeader {
				leader = (leader + 1) % len(ck.peers)
				break
			} else {
				ck.leader = leader
				return rpc.Err(firstReply.Err)
			}
		} else {
			break
		}
	}

	for {
		ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
		retryReply, err := ck.peers[leader].Put(ctx, args)
		cancel()
		retryOk := err == nil
		if retryOk {
			if rpc.Err(retryReply.Err) == rpc.ErrWrongLeader {
				leader = (leader + 1) % len(ck.peers)
			} else if rpc.Err(retryReply.Err) == rpc.ErrVersion {
				ck.leader = leader
				return rpc.ErrMaybe
			} else {
				ck.leader = leader
				return rpc.Err(retryReply.Err)
			}
		} else {
			leader = (leader + 1) % len(ck.peers)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (ck *Clerk) Signalings() ([]string, rpc.Err) {
	return ck.servers, rpc.OK
}
