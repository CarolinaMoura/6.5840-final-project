package raft

// The file ../raftapi/raftapi.go defines the interface that raft must
// expose to servers (or the tester), but see comments below for each
// of these functions for more details.
//
// In addition,  Make() creates a new raft peer that implements the
// raft interface.

import (
	"bytes"
	"context"
	"math/rand"
	"sync"
	"time"

	"6.5840-final-project/encoder"
	pb "6.5840-final-project/raft/raftpb"
)

type Persister interface {
	Save(state, snapshot []byte)
	ReadRaftState() []byte
	ReadSnapshot() []byte
	RaftStateSize() int
}

const (
	ABORT_TIMEOUT_MILISECONDS = 100
)

type ServerState int

const (
	Follower ServerState = iota
	Candidate
	Leader
)

type SnapshotState struct {
	Bytes []byte
	Index int32
	Term  int32
}

// A Go object implementing a single Raft peer.
type Raft struct {
	pb.UnimplementedRaftServer

	mu        sync.Mutex      // Lock to protect shared access to this peer's state
	peers     []pb.RaftClient // RPC end points of all peers
	persister Persister       // Object to hold this peer's persisted state
	me        int32           // this peer's index into peers[]

	// my state
	state      ServerState
	applyCh    chan ApplyMsg
	voteCount  int
	snapshot   SnapshotState
	appendCond *sync.Cond

	// elections, resets every tick
	hasReceivedHeartBeat bool

	// persistent state
	CurrentTerm  int32
	VotedFor     int32
	HasVotedTerm bool
	Log          Log

	// volatile state
	commitIndex int32
	lastApplied int32

	// volatile state on leaders
	nextIndex  []int32
	matchIndex []int32
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	term := int(rf.CurrentTerm)
	isleader := (rf.state == Leader)
	return term, isleader
}

func (rf *Raft) applyCommits() {
	applyMsgs := make([]ApplyMsg, 0)

	if rf.commitIndex < rf.lastApplied {
		panic("applied uncommitted entry")
	} else if rf.commitIndex == rf.lastApplied {
		return
	}

	for ; rf.lastApplied < rf.commitIndex; rf.lastApplied++ {
		applyMsg := ApplyMsg{
			CommandValid: true,
			CommandIndex: int(rf.lastApplied + 1),
			Command:      rf.Log.getEntry(rf.lastApplied + 1).Command,
		}
		applyMsgs = append(applyMsgs, applyMsg)
	}
	rf.lastApplied = rf.commitIndex

	for i := range applyMsgs {
		rf.applyCh <- applyMsgs[i]
	}
}

func (rf *Raft) updateLog(startIndex int32, entries []LogEntry, leaderCommit int32) {
	rf.Log.appendEntries(startIndex, entries)

	if leaderCommit > rf.commitIndex {
		indexOfLastNewEntry := startIndex + int32(len(entries)) - 1
		newCommitIndex := leaderCommit
		if indexOfLastNewEntry < newCommitIndex {
			newCommitIndex = indexOfLastNewEntry
		}
		if newCommitIndex <= rf.commitIndex {
			return
		}
		rf.commitIndex = newCommitIndex
	}

	rf.applyCommits()
}

// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
// before you've implemented snapshots, you should pass nil as the
// second argument to persister.Save().
// after you've implemented snapshots, pass the current snapshot
// (or nil if there's not yet a snapshot).
func (rf *Raft) persist() {
	w := new(bytes.Buffer)
	e := encoder.NewEncoder(w)
	e.Encode(rf.CurrentTerm)
	e.Encode(rf.VotedFor)
	e.Encode(rf.HasVotedTerm)
	e.Encode(rf.Log)
	raftstate := w.Bytes()
	rf.persister.Save(raftstate, rf.snapshot.Bytes)
}

// restore previously persisted state.
func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
	r := bytes.NewBuffer(data)
	d := encoder.NewDecoder(r)

	var currentTerm int32
	var votedFor int32
	var hasVotedTerm bool
	var log Log

	if d.Decode(&currentTerm) != nil ||
		d.Decode(&votedFor) != nil ||
		d.Decode(&hasVotedTerm) != nil ||
		d.Decode(&log) != nil {
		panic("decoding boom")
	} else {
		rf.CurrentTerm = currentTerm
		rf.VotedFor = votedFor
		rf.HasVotedTerm = hasVotedTerm
		rf.Log = log
	}
}

// how many bytes in Raft's persisted log?
func (rf *Raft) PersistBytes() int {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	return rf.persister.RaftStateSize()
}

// the service says it has created a snapshot that has
// all info up to and including index. this means the
// service no longer needs the log through (and including)
// that index. Raft should now trim its log as much as possible.
func (rf *Raft) Snapshot(index int, snapshot []byte) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	idx := int32(index)
	rf.snapshot.Bytes = snapshot
	rf.snapshot.Index = idx
	rf.snapshot.Term = rf.Log.getEntry(idx).Term
	rf.Log.rebase(idx)

	rf.persist()
}

func (rf *Raft) RequestVote(ctx context.Context, args *pb.RequestVoteArgs) (*pb.RequestVoteReply, error) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	reply := &pb.RequestVoteReply{
		Term:        rf.CurrentTerm,
		VoteGranted: false,
	}

	if args.Term > rf.CurrentTerm {
		rf.toFollower(args.Term)
		reply.Term = rf.CurrentTerm
	}

	hasVotedTermForSomeoneElse := rf.HasVotedTerm && (rf.VotedFor != args.CandidateId)

	if args.Term < rf.CurrentTerm || hasVotedTermForSomeoneElse {
		return reply, nil
	}

	lastLogTerm := rf.Log.getLastLogTerm()
	if lastLogTerm == -1 {
		panic("missing last log term")
	}

	if args.LastLogTerm < lastLogTerm {
		return reply, nil
	}

	if args.LastLogTerm == lastLogTerm && args.LastLogIndex < rf.Log.getLastLogIndex() {
		return reply, nil
	}

	reply.VoteGranted = true
	rf.HasVotedTerm = true
	rf.VotedFor = args.CandidateId
	rf.hasReceivedHeartBeat = true
	rf.persist()
	return reply, nil
}

func (rf *Raft) AppendEntries(ctx context.Context, args *pb.AppendEntriesArgs) (*pb.AppendEntriesReply, error) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	reply := &pb.AppendEntriesReply{
		Term:    rf.CurrentTerm,
		Success: false,
	}

	if args.Term > rf.CurrentTerm {
		rf.toFollower(args.Term)
		reply.Term = rf.CurrentTerm
	} else if args.Term < rf.CurrentTerm {
		return reply, nil
	}

	if rf.state == Candidate {
		rf.state = Follower
	}

	rf.hasReceivedHeartBeat = true

	prevEntryTerm := rf.Log.getEntry(args.PrevLogIndex).Term
	if prevEntryTerm != args.PrevLogTerm {
		reply.Success = false
		reply.ConflictingTerm = prevEntryTerm
		reply.ConflictingTermStart = rf.Log.termStart(prevEntryTerm)
		reply.LogLength = rf.Log.length()
		return reply, nil
	}

	reply.Success = true
	rf.updateLog(args.PrevLogIndex+1, fromPbEntries(args.Entries), args.LeaderCommit)
	if len(args.Entries) > 0 {
		rf.persist()
	}
	return reply, nil
}

func (rf *Raft) InstallSnapshot(ctx context.Context, args *pb.InstallSnapshotArgs) (*pb.InstallSnapshotReply, error) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	reply := &pb.InstallSnapshotReply{
		Term: rf.CurrentTerm,
	}

	if args.Term < rf.CurrentTerm {
		return reply, nil
	} else if args.Term > rf.CurrentTerm {
		rf.toFollower(args.Term)
		reply.Term = rf.CurrentTerm
	}

	if args.Snapshot.Index < rf.Log.BaseIndex || args.Snapshot.Index < rf.commitIndex {
		return reply, nil
	}

	rf.snapshot = SnapshotState{
		Bytes: args.Snapshot.Bytes,
		Index: args.Snapshot.Index,
		Term:  args.Snapshot.Term,
	}

	entry := rf.Log.getEntry(rf.snapshot.Index)
	rf.Log.rebase(rf.snapshot.Index)
	if entry.Term == -1 || entry.Term != rf.snapshot.Term {
		rf.Log.truncate(rf.snapshot.Index)
	}
	baseEntry := LogEntry{
		Term:    rf.snapshot.Term,
		Command: nil,
	}
	if len(rf.Log.Entries) == 0 {
		rf.Log.Entries = append(rf.Log.Entries, baseEntry)
	} else {
		rf.Log.Entries[0] = baseEntry
	}

	rf.persist()

	rf.commitIndex = rf.snapshot.Index
	rf.lastApplied = rf.snapshot.Index

	applyMsg := ApplyMsg{
		SnapshotValid: true,
		Snapshot:      rf.snapshot.Bytes,
		SnapshotTerm:  int(rf.snapshot.Term),
		SnapshotIndex: int(rf.snapshot.Index),
	}

	rf.applyCh <- applyMsg
	return reply, nil
}

func (rf *Raft) sendRequestVote(server int, args *pb.RequestVoteArgs) (*pb.RequestVoteReply, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), ABORT_TIMEOUT_MILISECONDS*time.Millisecond)
	defer cancel()
	reply, err := rf.peers[server].RequestVote(ctx, args)
	return reply, err == nil
}

func (rf *Raft) sendAppendEntries(server int, args *pb.AppendEntriesArgs) (*pb.AppendEntriesReply, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), ABORT_TIMEOUT_MILISECONDS*time.Millisecond)
	defer cancel()
	reply, err := rf.peers[server].AppendEntries(ctx, args)
	return reply, err == nil
}

func (rf *Raft) sendInstallSnapshot(server int, args *pb.InstallSnapshotArgs) (*pb.InstallSnapshotReply, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), ABORT_TIMEOUT_MILISECONDS*time.Millisecond)
	defer cancel()
	reply, err := rf.peers[server].InstallSnapshot(ctx, args)
	return reply, err == nil
}

// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	index := -1

	term := rf.CurrentTerm
	isLeader := rf.state == Leader

	if isLeader {
		index = int(rf.Log.getLastLogIndex()) + 1
		rf.Log.appendEntry(LogEntry{Term: term, Command: command})
		rf.persist()
		rf.appendCond.Broadcast()
	}

	return index, int(term), isLeader
}

func (rf *Raft) ticker() {
	for true {
		ms := 500 + (rand.Int63() % 1000)
		time.Sleep(time.Duration(ms) * time.Millisecond)

		rf.mu.Lock()

		isleader := rf.state == Leader
		if !(isleader || rf.hasReceivedHeartBeat) {
			rf.toCandidate()
		}
		rf.hasReceivedHeartBeat = false
		rf.mu.Unlock()
	}
}

// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
func Make(peers []pb.RaftClient, me int,
	persister Persister, applyCh chan ApplyMsg) RaftAPI {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = int32(me)
	rf.appendCond = sync.NewCond(&rf.mu)

	rf.Log.appendEntry(LogEntry{Term: 0})

	rf.state = Follower
	rf.applyCh = make(chan ApplyMsg, 10000)
	go func() {
		var applyMsg ApplyMsg
		for {
			applyMsg = <-rf.applyCh
			applyCh <- applyMsg
		}
	}()

	rf.readPersist(persister.ReadRaftState())
	rf.Snapshot(int(rf.Log.BaseIndex), persister.ReadSnapshot())
	rf.commitIndex = rf.Log.BaseIndex
	rf.lastApplied = rf.Log.BaseIndex

	go rf.ticker()

	return rf
}
