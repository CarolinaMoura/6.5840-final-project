package rsm

import (
	"sync"
	"time"

	"6.5840-final-project/raft"
	"6.5840-final-project/raft/raftpb"
	rpc "6.5840-final-project/rsm/rpc"
)

type Op struct {
	// Your definitions here.
	// Field names must start with capital letters,
	// otherwise RPC will break.
	Me  int
	Req any
}

// A server (i.e., ../server.go) that wants to replicate itself calls
// MakeRSM and must implement the StateMachine interface.  This
// interface allows the rsm package to interact with the server for
// server-specific operations: the server must implement DoOp to
// execute an operation (e.g., a Get or Put request), and
// Snapshot/Restore to snapshot and restore the server's state.
type StateMachine interface {
	DoOp(any) any
	Snapshot() []byte
	Restore([]byte)
}

type RSM struct {
	mu           sync.Mutex
	me           int
	rf           raft.RaftAPI
	applyCh      chan raft.ApplyMsg
	maxraftstate int // snapshot if log grows this big
	sm           StateMachine
	// Your definitions here.
	submitCh    map[int]chan any
	commitIndex int
}

// servers[] contains the ports of the set of
// servers that will cooperate via Raft to
// form the fault-tolerant key/value service.
//
// me is the index of the current server in servers[].
//
// the k/v server should store snapshots through the underlying Raft
// implementation, which should call persister.SaveStateAndSnapshot() to
// atomically save the Raft state along with the snapshot.
// The RSM should snapshot when Raft's saved state exceeds maxraftstate bytes,
// in order to allow Raft to garbage-collect its log. if maxraftstate is -1,
// you don't need to snapshot.
//
// MakeRSM() must return quickly, so it should start goroutines for
// any long-running work.
func MakeRSM(servers []raftpb.RaftClient, me int, persister raft.Persister, maxraftstate int, sm StateMachine) *RSM {
	rsm := &RSM{
		me:           me,
		maxraftstate: maxraftstate,
		applyCh:      make(chan raft.ApplyMsg),
		sm:           sm,
		submitCh:     make(map[int]chan any),
		commitIndex:  0,
	}
	rsm.rf = raft.Make(servers, me, persister, rsm.applyCh)
	if snap := persister.ReadSnapshot(); len(snap) > 0 {
		rsm.sm.Restore(snap)
	}
	go rsm.reader()
	go rsm.leaderCheck()
	if maxraftstate != -1 {
		go rsm.persistCheck()
	}
	return rsm
}

func (rsm *RSM) Raft() raft.RaftAPI {
	return rsm.rf
}

func (rsm *RSM) persistCheck() {
	for {
		rsm.mu.Lock()
		// check commitIndex isnt 0 -> instance has received at least one commit before snapping
		if rsm.rf.PersistBytes() > rsm.maxraftstate && rsm.commitIndex != 0 {
			snapshot := rsm.sm.Snapshot()
			commitIndex := rsm.commitIndex
			rsm.rf.Snapshot(commitIndex, snapshot)
		}
		rsm.mu.Unlock()

		time.Sleep(100 * time.Millisecond)
	}
}

func (rsm *RSM) leaderCheck() {
	for {
		_, isLeader := rsm.rf.GetState()
		if !isLeader {
			rsm.mu.Lock()
			cancelChs := make([]chan any, 0)
			for _, submitCh := range rsm.submitCh {
				cancelChs = append(cancelChs, submitCh)
			}
			rsm.submitCh = make(map[int]chan any)
			rsm.mu.Unlock()

			for _, cancelCh := range cancelChs {
				cancelCh <- nil
			}
		}

		time.Sleep(100 * time.Millisecond)
	}
}

func (rsm *RSM) reader() {
	rsm.mu.Lock()
	applyCh := rsm.applyCh
	rsm.mu.Unlock()

	for {
		commit := <-applyCh
		rsm.mu.Lock()

		if commit.CommandValid {
			command, _ := commit.Command.(Op)
			rsm.commitIndex = commit.CommandIndex

			submitCh, submitted := rsm.submitCh[commit.CommandIndex]
			// delete(rsm.submitCh, commit.CommandIndex)

			res := rsm.sm.DoOp(command.Req)
			if submitted && command.Me == rsm.me {
				submitCh <- res
			}
		} else if commit.SnapshotValid {
			rsm.commitIndex = max(rsm.commitIndex, commit.SnapshotIndex)
			rsm.sm.Restore(commit.Snapshot)
		} else {
			panic("invalid apply msg")
		}

		rsm.mu.Unlock()
	}
}

// Submit a command to Raft, and wait for it to be committed.  It
// should return ErrWrongLeader if client should find new leader and
// try again.
func (rsm *RSM) Submit(req any) (rpc.Err, any) {

	// Submit creates an Op structure to run a command through Raft;
	// for example: op := Op{Me: rsm.me, Id: id, Req: req}, where req
	// is the argument to Submit and id is a unique id for the op.

	rsm.mu.Lock()

	op := Op{
		Me:  rsm.me,
		Req: req,
	}

	index, _, isLeader := rsm.Raft().Start(op)

	if !isLeader {
		rsm.mu.Unlock()
		return rpc.ErrWrongLeader, nil // i'm dead, try another server.
	}

	submitCh := make(chan any, 1)
	rsm.submitCh[index] = submitCh

	rsm.mu.Unlock()

	res := <-submitCh

	if res == nil {
		return rpc.ErrWrongLeader, nil
	}

	return rpc.OK, res
}
