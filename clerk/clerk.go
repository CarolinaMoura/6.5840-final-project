package clerk

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"6.5840-final-project/kv/kvpb"
	rpc "6.5840-final-project/rsm/rpc"

	"google.golang.org/grpc"
)

const (
	callTimeout = 5 * time.Second

	// heartbeatsKey holds a CSV map of addr=unixMillis. Each node updates
	// its own row periodically; Signalings() filters out rows whose
	// timestamp is older than the freshness window.
	heartbeatsKey = "heartbeats"
)

type Clerk struct {
	peers  []kvpb.KVClient // gRPC stubs, one per kv peer
	leader int             // last successful leader (index into peers[])
}

// conns: list of grpc client connections to each peer.
// servers: list of advertise addresses for each peer
// Returns a clerk. The caller owns conns and is responsible for closing them on shutdown.
func MakeClerk(conns []*grpc.ClientConn) (*Clerk, error) {
	if len(conns) == 0 {
		return nil, fmt.Errorf("MakeClerk: conns is empty")
	}
	peers := make([]kvpb.KVClient, len(conns))
	for i, c := range conns {
		peers[i] = kvpb.NewKVClient(c)
	}
	return &Clerk{peers: peers}, nil
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

// Returns the advertise addresses of every signaling whose last
// heartbeat is fresh (within ttl from now).
func (ck *Clerk) Signalings() ([]string, rpc.Err) {
	raw, _, err := ck.Get(heartbeatsKey)
	if err == rpc.ErrNoKey {
		return []string{}, rpc.OK
	}
	if err != rpc.OK {
		return nil, err
	}
	beats := parseHeartbeats(raw)
	nowMs := time.Now().UnixMilli()
	const freshnessMs int64 = 30_000
	out := []string{}
	for addr, ts := range beats {
		if nowMs-ts <= freshnessMs {
			out = append(out, addr)
		}
	}
	return out, rpc.OK
}

// Starts a goroutine that heartbeats this node's address
// into the heartbeats map every ttl/3 seconds. The first
// heartbeat fires immediately so the node is visible right away. Returns
// a stop function that cancels the goroutine.
func (ck *Clerk) RegisterWithLease(addr string, ttlSeconds int64) func() {
	if ttlSeconds <= 0 {
		ttlSeconds = 30
	}
	interval := time.Duration(ttlSeconds) * time.Second / 3
	if interval < time.Second {
		interval = time.Second
	}
	done := make(chan struct{})
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			ck.heartbeat(addr)
			select {
			case <-done:
				return
			case <-t.C:
			}
		}
	}()
	return func() { close(done) }
}

// Sets heartbeats[addr] = now. On
// version conflict (another node heartbeated concurrently) it retries
// until success.
func (ck *Clerk) heartbeat(addr string) {
	now := time.Now().UnixMilli()
	for {
		raw, version, err := ck.Get(heartbeatsKey)
		if err != rpc.OK && err != rpc.ErrNoKey {
			return
		}
		beats := map[string]int64{}
		if err == rpc.OK {
			beats = parseHeartbeats(raw)
		} else {
			version = 0
		}
		beats[addr] = now
		putErr := ck.Put(heartbeatsKey, serializeHeartbeats(beats), version)
		if putErr == rpc.OK || putErr == rpc.ErrMaybe {
			return
		}
		if putErr == rpc.ErrVersion {
			continue // raced
		}
		return
	}
}

// parseHeartbeats decodes "addr1=ts1,addr2=ts2,..." into a map.
// Malformed entries are skipped.
func parseHeartbeats(raw string) map[string]int64 {
	out := map[string]int64{}
	if raw == "" {
		return out
	}
	for _, item := range strings.Split(raw, ",") {
		eq := strings.IndexByte(item, '=')
		if eq <= 0 {
			continue
		}
		ts, err := strconv.ParseInt(item[eq+1:], 10, 64)
		if err != nil {
			continue
		}
		out[item[:eq]] = ts
	}
	return out
}

func serializeHeartbeats(m map[string]int64) string {
	parts := make([]string, 0, len(m))
	for addr, ts := range m {
		parts = append(parts, addr+"="+strconv.FormatInt(ts, 10))
	}
	return strings.Join(parts, ",")
}
