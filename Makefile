proto:
	protoc \
		--go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		raft/raftpb/raft.proto

test-server:
	go test . -v

test-raft:
	go test ./raft -v

test:
	make test-server
	make test-raft