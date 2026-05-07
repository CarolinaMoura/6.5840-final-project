proto:
	protoc \
		--go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		raft/raftpb/raft.proto kv/kvpb/kv.proto

test-raft:
	go test ./raft -v

test:
	go test . -v
	npx vitest run

init:
	npm install --prefix web 
	mkdir -p data/node1 data/node2 data/node3
	# automate using constants for N and M

cluster:
	npm run build --prefix web 
	docker compose up --build



