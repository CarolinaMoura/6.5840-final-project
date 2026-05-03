package raft

import (
	"fmt"
)

func (rf *Raft) toFollower(newTerm int32) {
	if newTerm <= rf.CurrentTerm {
		panic(fmt.Errorf("Server#%v bad toFollower %v -> %v", rf.me, rf.CurrentTerm, newTerm))
	}

	rf.state = Follower
	rf.hasReceivedHeartBeat = false
	rf.CurrentTerm = newTerm
	rf.HasVotedTerm = false

	rf.persist()
}
