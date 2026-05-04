proto:
	protoc \
		--go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		raft/raftpb/raft.proto

test-raft:
	go test ./raft -v

test:
	go test . -v
	npx vitest run

init:
	npm install --prefix web 

cluster:
	npm run build --prefix web 
	docker compose up --build



