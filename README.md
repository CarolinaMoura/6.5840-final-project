## TODO

- Document limitation for users behind symmetric NAT or spin up Cloudflare TURN.

## App

To start, run `npm install` from the root dir to install the dependencies for both the Go server and the React frontend.

## Server

### Running

To spin up a server on :8080, run the following command from the root dir:

```
npm run dev
```

### API endpoints:

| Route               | Method | Behavior                                  |
| ------------------- | ------ | ----------------------------------------- |
| /                   | GET    | serves the main page                      |
| /api/rooms          | POST   | creates a new room and returns its name.  |
| /api/ws/rooms/:room | GET    | upgrades to WebSocket and joins the room. |

## RPC definitions

### raftpb

If you change `raft.proto`, run `make proto` to generate the new gRPC stubs. If that fails, you may need to install protoc and the protoc-gen-go plugin.

## Testing

To run all tests, run the following command from the root dir:

```
make test
```
