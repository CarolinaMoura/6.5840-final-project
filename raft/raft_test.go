package raft

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	pb "6.5840-final-project/raft/raftpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

// memPersister is an in-memory Persister for tests.
type memPersister struct {
	mu       sync.Mutex
	state    []byte
	snapshot []byte
}

func (m *memPersister) Save(state, snapshot []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = append([]byte(nil), state...)
	m.snapshot = append([]byte(nil), snapshot...)
}

func (m *memPersister) ReadRaftState() []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]byte(nil), m.state...)
}

func (m *memPersister) ReadSnapshot() []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]byte(nil), m.snapshot...)
}

func (m *memPersister) RaftStateSize() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.state)
}

// cluster spins up n in-process Raft peers connected via bufconn.
type cluster struct {
	t           *testing.T
	n           int
	listeners   []*bufconn.Listener
	servers     []*grpc.Server
	rafts       []*Raft
	applyChs    []chan ApplyMsg
	clientConns [][]*grpc.ClientConn // clientConns[i][j] = peer i's outgoing conn to peer j
}

func makeCluster(t *testing.T, n int) *cluster {
	c := &cluster{t: t, n: n}
	c.listeners = make([]*bufconn.Listener, n)
	c.servers = make([]*grpc.Server, n)
	c.rafts = make([]*Raft, n)
	c.applyChs = make([]chan ApplyMsg, n)
	c.clientConns = make([][]*grpc.ClientConn, n)

	for i := 0; i < n; i++ {
		c.listeners[i] = bufconn.Listen(1 << 20)
	}

	for i := 0; i < n; i++ {
		peers := make([]pb.RaftClient, n)
		c.clientConns[i] = make([]*grpc.ClientConn, n)
		for j := 0; j < n; j++ {
			jj := j
			conn, err := grpc.NewClient(
				"passthrough://bufnet",
				grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
					return c.listeners[jj].DialContext(ctx)
				}),
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			if err != nil {
				t.Fatalf("dial peer %d: %v", j, err)
			}
			c.clientConns[i][j] = conn
			peers[j] = pb.NewRaftClient(conn)
		}

		c.applyChs[i] = make(chan ApplyMsg, 1024)
		c.rafts[i] = Make(peers, i, &memPersister{}, c.applyChs[i]).(*Raft)
		c.servers[i] = grpc.NewServer()
		pb.RegisterRaftServer(c.servers[i], c.rafts[i])

		ii := i
		go func() {
			_ = c.servers[ii].Serve(c.listeners[ii])
		}()
	}

	return c
}

func (c *cluster) cleanup() {
	for _, conns := range c.clientConns {
		for _, conn := range conns {
			if conn != nil {
				_ = conn.Close()
			}
		}
	}
	for _, s := range c.servers {
		if s != nil {
			s.Stop()
		}
	}
	for _, l := range c.listeners {
		if l != nil {
			_ = l.Close()
		}
	}
}

// stopPeer fully isolates peer i: stops its server (no incoming RPCs)
// and closes its outgoing connections (no outgoing RPCs either).
func (c *cluster) stopPeer(i int) {
	c.servers[i].Stop()
	for _, conn := range c.clientConns[i] {
		if conn != nil {
			_ = conn.Close()
		}
	}
}

// countLeaders returns how many peers currently believe they're the leader.
func (c *cluster) countLeaders() int {
	count := 0
	for _, r := range c.rafts {
		if _, isLeader := r.GetState(); isLeader {
			count++
		}
	}
	return count
}

// findLeader returns the index of the (presumed unique) current leader, or -1.
func (c *cluster) findLeader() int {
	for i, r := range c.rafts {
		if _, isLeader := r.GetState(); isLeader {
			return i
		}
	}
	return -1
}

// waitForLeader polls until exactly one peer (among aliveSet, or all if nil)
// reports leadership, or fails the test on timeout.
func (c *cluster) waitForLeader(timeout time.Duration) int {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if c.countLeaders() == 1 {
			return c.findLeader()
		}
		time.Sleep(20 * time.Millisecond)
	}
	c.t.Fatalf("no leader elected within %v", timeout)
	return -1
}

func TestLeaderElection(t *testing.T) {
	c := makeCluster(t, 3)
	defer c.cleanup()

	leader := c.waitForLeader(3 * time.Second)
	t.Logf("elected leader: peer %d", leader)

	// Stays stable.
	time.Sleep(500 * time.Millisecond)
	if c.countLeaders() != 1 {
		t.Fatalf("expected 1 leader after settling, got %d", c.countLeaders())
	}
}

func TestLogReplication(t *testing.T) {
	c := makeCluster(t, 3)
	defer c.cleanup()

	leader := c.waitForLeader(3 * time.Second)

	idx, _, ok := c.rafts[leader].Start("hello")
	if !ok {
		t.Fatalf("Start returned !isLeader on leader %d", leader)
	}
	t.Logf("leader %d accepted command at index %d", leader, idx)

	// Each peer's applyCh should receive the command.
	deadline := time.Now().Add(3 * time.Second)
	got := make(map[int]any)
	for time.Now().Before(deadline) && len(got) < c.n {
		for i, ch := range c.applyChs {
			if _, ok := got[i]; ok {
				continue
			}
			select {
			case msg := <-ch:
				if msg.CommandValid && msg.CommandIndex == idx {
					got[i] = msg.Command
				}
			default:
			}
		}
		time.Sleep(20 * time.Millisecond)
	}

	if len(got) < c.n {
		t.Fatalf("only %d/%d peers applied the command", len(got), c.n)
	}
	for i, cmd := range got {
		if s, _ := cmd.(string); s != "hello" {
			t.Fatalf("peer %d got %#v, want \"hello\"", i, cmd)
		}
	}
}

func TestLeaderChange(t *testing.T) {
	c := makeCluster(t, 3)
	defer c.cleanup()

	first := c.waitForLeader(3 * time.Second)
	t.Logf("first leader: peer %d", first)

	// Take the leader offline. The remaining 2 form a quorum and must elect a new leader.
	c.stopPeer(first)

	// Poll for a new leader different from the stopped peer.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		newLeader := -1
		count := 0
		for i, r := range c.rafts {
			if i == first {
				continue
			}
			if _, isLeader := r.GetState(); isLeader {
				count++
				newLeader = i
			}
		}
		if count == 1 && newLeader != first {
			t.Logf("new leader: peer %d", newLeader)
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("no new leader elected after stopping peer %d", first)
}
