## Testing
To run all tests in the main package, run the following command from the root dir:
```
go test -v .
```

## Server
API endpoints:

| Route | Method | Behavior
| ----- | ------ | --------
| /     | GET    | serves the main page
| /rooms | POST   | creates a new room and returns its name.
| /ws/rooms/:room | GET | upgrades to WebSocket and joins the room.


To spin up a server on :8080
```
go run server.go
```