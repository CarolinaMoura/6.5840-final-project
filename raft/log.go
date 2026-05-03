package raft

import (
	"bytes"

	"6.5840-final-project/encoder"
	pb "6.5840-final-project/raft/raftpb"
)

type LogEntry struct {
	Term    int32
	Command any
}

type Log struct {
	BaseIndex int32
	Entries   []LogEntry
}

func (log *Log) getLastLogIndex() int32 {
	return log.BaseIndex + int32(len(log.Entries)) - 1
}

func (log *Log) getEntry(index int32) LogEntry {
	indexRebased := index - log.BaseIndex
	if index < 0 || index > log.getLastLogIndex() || indexRebased < 0 {
		return LogEntry{Term: -1}
	}
	return log.Entries[indexRebased]
}

func (log *Log) getLastLogTerm() int32 {
	return log.getEntry(log.getLastLogIndex()).Term
}

func (log *Log) getEntriesFrom(index int32) []LogEntry {
	rebased := index - log.BaseIndex
	res := make([]LogEntry, 0)
	return append(res, log.Entries[rebased:]...)
}

func (log *Log) truncate(startIndex int32) {
	rebased := startIndex - log.BaseIndex
	log.Entries = log.Entries[0:rebased]
}

func (log *Log) appendEntry(entry LogEntry) {
	log.Entries = append(log.Entries, entry)
}

// truncates the log at `startIndex` and then appends `entries`
func (log *Log) appendEntries(startIndex int32, entries []LogEntry) {
	for i, entry := range entries {
		rebasedIndex := startIndex + int32(i)

		if rebasedIndex > log.getLastLogIndex() {
			for _, entry := range entries[i:] {
				log.appendEntry(entry)
			}
			return
		}

		if log.getEntry(rebasedIndex).Term != entry.Term {
			log.truncate(rebasedIndex)
			for _, entry := range entries[i:] {
				log.appendEntry(entry)
			}
			return
		}
	}
}

// returns the index of the first entry in the log for the given term, or -1 if none
func (log *Log) termStart(term int32) int32 {
	for i := range log.Entries {
		if log.Entries[i].Term == term {
			return log.BaseIndex + int32(i)
		}
	}
	return -1
}

// returns the index of the last entry in the log for the given term, or -1 if none
func (log *Log) termEnd(term int32) int32 {
	for i := len(log.Entries) - 1; i >= 0; i-- {
		if log.Entries[i].Term == term {
			return log.BaseIndex + int32(i)
		}
	}
	return -1
}

func (log *Log) length() int32 {
	return log.BaseIndex + int32(len(log.Entries))
}

func (log *Log) rebase(index int32) {
	if index < log.BaseIndex {
		panic("tried rebasing to earlier index")
	}
	indexRebased := index - log.BaseIndex
	if int(indexRebased) >= len(log.Entries) {
		log.Entries = make([]LogEntry, 0)
	} else {
		log.Entries = log.Entries[indexRebased:]
	}
	log.BaseIndex = index
}

// toPbEntry encodes a Go LogEntry (with arbitrary Command) to its proto form
// by gob-encoding the Command into bytes.
func toPbEntry(e LogEntry) *pb.LogEntry {
	var buf bytes.Buffer
	if e.Command != nil {
		enc := encoder.NewEncoder(&buf)
		enc.Encode(&e.Command)
	}
	return &pb.LogEntry{Term: e.Term, Command: buf.Bytes()}
}

// fromPbEntry is the inverse of toPbEntry.
func fromPbEntry(pe *pb.LogEntry) LogEntry {
	var cmd any
	if len(pe.Command) > 0 {
		dec := encoder.NewDecoder(bytes.NewReader(pe.Command))
		dec.Decode(&cmd)
	}
	return LogEntry{Term: pe.Term, Command: cmd}
}

func toPbEntries(es []LogEntry) []*pb.LogEntry {
	out := make([]*pb.LogEntry, len(es))
	for i, e := range es {
		out[i] = toPbEntry(e)
	}
	return out
}

func fromPbEntries(es []*pb.LogEntry) []LogEntry {
	out := make([]LogEntry, len(es))
	for i, e := range es {
		out[i] = fromPbEntry(e)
	}
	return out
}
