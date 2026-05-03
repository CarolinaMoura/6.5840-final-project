package raft

import (
	pb "6.5840-final-project/raft/raftpb"
)

func (rf *Raft) requestVote(peerId int) {
	rf.mu.Lock()
	args := &pb.RequestVoteArgs{
		Term:         rf.CurrentTerm,
		CandidateId:  rf.me,
		LastLogIndex: rf.Log.getLastLogIndex(),
		LastLogTerm:  rf.Log.getLastLogTerm(),
	}
	rf.mu.Unlock()

	reply, ok := rf.sendRequestVote(peerId, args)

	rf.mu.Lock()
	defer rf.mu.Unlock()

	if !ok {
		return
	}

	if reply.Term > rf.CurrentTerm {
		rf.toFollower(reply.Term)
		return
	}

	if rf.state != Candidate {
		return
	}

	// ignore replies from other candidacies
	if reply.Term != args.Term {
		return
	}

	if reply.Term == rf.CurrentTerm && reply.VoteGranted {
		rf.voteCount++
		if rf.voteCount == ((len(rf.peers) / 2) + 1) {
			rf.toLeader()
		}
	}
}

func (rf *Raft) toCandidate() {
	rf.state = Candidate
	rf.voteCount = 1
	rf.CurrentTerm = rf.CurrentTerm + 1
	rf.HasVotedTerm = true
	rf.VotedFor = rf.me
	rf.persist()

	for peerId := range rf.peers {
		if int32(peerId) == rf.me {
			continue
		}

		go rf.requestVote(peerId)
	}
}
