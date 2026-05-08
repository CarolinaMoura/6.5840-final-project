# Cluster

## Prerequisites

Run `make init` from the root dir to install the dependencies for both the Go server and the React frontend.

## Running

### Docker daemon

The cluster runs in Docker, so make sure the Docker daemon is up before `make cluster`. Verify with:

```
docker info
```

If it prints engine info you're good; if it errors with "failed to connect to the docker API", Docker isn't running yet.

### Building the cluster

From the root dir:

```
make cluster
```
That will spin-up 2 nodes. You can change this number by modifying the `docker-compose.yaml` file.

Open the app at <http://localhost:8081> or <http://localhost:8082>. Both signalings serve the same frontend; rooms are coordinated through etcd so they don't need to know about each other directly.

Each signaling reads `ADVERTISE_ADDR` (the host:port browsers should use to reach it) from its environment and registers itself in etcd under `server/<addr>` with a 10-second lease. The lease auto-renews while the process is alive and expires shortly after a crash, so the live set updates itself.

## Architecture

- **Room creation (`POST /api/rooms`)**: picks a random live signaling from `server/*`, writes `room/<id> -> <addr>` in etcd, returns `{room, server}` to the client.
- **Room lookup (`GET /api/rooms/:room`)**: reads `room/<id>` from etcd. If the recorded server is no longer in the live set (lease expired), reassigns the room to a live signaling and returns the new addr.
- **Joining (`GET /api/ws/rooms/:room`)**: a `wsGate` middleware validates the room and rejects with 404 if this signaling isn't the assigned one; otherwise the WS is upgraded.
- **Client reconnect**: on WS close, the client re-fetches `/api/rooms/:room` (which reassigns the server if previous died) and reconnects with exponential backoff. The RTC instance is kept alive across signaling drops so existing WebRTC peer connections survive.

## API endpoints

| Route               | Method | Behavior                                                                                 |
| ------------------- | ------ | ---------------------------------------------------------------------------------------- |
| /                   | GET    | serves the main page                                                                     |
| /api/rooms          | POST   | creates a new room; returns `{room, server}` (the signaling assigned to host it)         |
| /api/rooms/:room    | GET    | returns `{server}` for a room; reassigns to a live signaling if the recorded one died    |
| /api/ws/rooms/:room | GET    | upgrades to WebSocket and joins the room (rejects with 404 if served by wrong signaling) |

# gRPC definitions

## raftpb

If you change `raft.proto` or `kv.proto`, run `make proto` to generate the new gRPC stubs. If that fails, you may need to install protoc and the protoc-gen-go plugin.

# Testing

To run all tests, run the following command from the root dir:

```
make test
```
