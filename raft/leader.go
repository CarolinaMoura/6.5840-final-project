package raft

import (
	"time"

	pb "6.5840-final-project/raft/raftpb"
)

func (rf *Raft) updateLeaderCommit() {
	newCommitIndex := rf.commitIndex
	for i := rf.commitIndex + 1; i <= rf.Log.getLastLogIndex(); i++ {
		if rf.Log.getEntry(i).Term != rf.CurrentTerm {
			continue
		}
		count := 0
		for j := range rf.peers {
			if int32(j) == rf.me || rf.matchIndex[j] >= i {
				count++
			}
		}

		if count > len(rf.peers)/2 {
			newCommitIndex = i
		}
	}

	if rf.commitIndex == newCommitIndex {
		return
	}

	rf.commitIndex = newCommitIndex
	rf.applyCommits()
}

func (rf *Raft) getAppendEntriesArgsForFollower(peerId int) *pb.AppendEntriesArgs {
	prevLogIndex := rf.nextIndex[peerId] - 1
	return &pb.AppendEntriesArgs{
		Term:         rf.CurrentTerm,
		LeaderId:     rf.me,
		PrevLogIndex: prevLogIndex,
		PrevLogTerm:  rf.Log.getEntry(prevLogIndex).Term,
		Entries:      toPbEntries(rf.Log.getEntriesFrom(prevLogIndex + 1)),
		LeaderCommit: rf.commitIndex,
	}
}

// returns false when no longer leader
func (rf *Raft) leaderInstallL(peerId int, toLeaderTerm int32) bool {
	snapshotArgs := &pb.InstallSnapshotArgs{
		Term:     rf.CurrentTerm,
		LeaderId: rf.me,
		Snapshot: &pb.SnapshotState{
			Bytes: rf.snapshot.Bytes,
			Index: rf.snapshot.Index,
			Term:  rf.snapshot.Term,
		},
	}

	rf.mu.Unlock()
	reply, ok := rf.sendInstallSnapshot(peerId, snapshotArgs)
	rf.mu.Lock()

	if !ok {
		return rf.state == Leader && rf.CurrentTerm == toLeaderTerm
	}

	if reply.Term > rf.CurrentTerm {
		rf.toFollower(reply.Term)
		return false
	}

	if rf.state != Leader || toLeaderTerm != rf.CurrentTerm {
		return false
	}

	rf.nextIndex[peerId] = rf.snapshot.Index + 1
	rf.matchIndex[peerId] = rf.snapshot.Index
	rf.updateLeaderCommit()

	return true
}

// returns false when no longer leader
func (rf *Raft) updateFollower(peerId int, toLeaderTerm int32) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	for rf.CurrentTerm == toLeaderTerm {
		for rf.nextIndex[peerId]-1 < rf.Log.BaseIndex { // missing entry, repeat until good
			stillLeader := rf.leaderInstallL(peerId, toLeaderTerm)
			if !stillLeader {
				return
			}
		}

		args := rf.getAppendEntriesArgsForFollower(peerId)

		rf.mu.Unlock()
		reply, ok := rf.sendAppendEntries(peerId, args)
		if !ok {
			rf.mu.Lock()
			continue
		}
		rf.mu.Lock()

		if reply.Term > rf.CurrentTerm {
			rf.toFollower(reply.Term)
			return
		}

		if rf.state != Leader || toLeaderTerm != rf.CurrentTerm {
			return
		}

		if reply.Success {
			rf.nextIndex[peerId] = args.PrevLogIndex + int32(len(args.Entries)) + 1
			rf.matchIndex[peerId] = rf.nextIndex[peerId] - 1
			rf.updateLeaderCommit()

			if rf.matchIndex[peerId] == rf.Log.getLastLogIndex() {
				rf.appendCond.Wait()
			}
		} else {
			if reply.LogLength-1 < args.PrevLogIndex {
				rf.nextIndex[peerId] = reply.LogLength
			} else {
				if reply.ConflictingTerm == -1 {
					stillLeader := rf.leaderInstallL(peerId, toLeaderTerm)
					if !stillLeader {
						return
					}
					continue
				}

				hasTerm := rf.Log.termStart(reply.ConflictingTerm) != -1
				if hasTerm {
					rf.nextIndex[peerId] = rf.Log.termEnd(reply.ConflictingTerm) + 1
				} else {
					rf.nextIndex[peerId] = reply.ConflictingTermStart
				}
			}
		}
	}
}

func (rf *Raft) heartbeat(toLeaderTerm int32) {
	for {
		time.Sleep(time.Duration(100) * time.Millisecond)
		term, _ := rf.GetState()
		if int32(term) > toLeaderTerm {
			return
		}
		rf.appendCond.Broadcast()
	}
}

func (rf *Raft) toLeader() {
	rf.nextIndex = make([]int32, len(rf.peers))
	rf.matchIndex = make([]int32, len(rf.peers))
	for i := range rf.peers {
		rf.nextIndex[i] = rf.Log.getLastLogIndex() + 1
		rf.matchIndex[i] = 0
	}

	rf.state = Leader

	for i := range rf.peers {
		if int32(i) == rf.me {
			continue
		}
		go rf.updateFollower(i, rf.CurrentTerm)
	}

	go rf.heartbeat(rf.CurrentTerm)
}
