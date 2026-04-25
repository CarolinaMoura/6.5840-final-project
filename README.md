
## Server
### Running
To spin up a server on :8080, run the following command from the root dir:
```
npm run dev
```

### API endpoints:

| Route | Method | Behavior
| ----- | ------ | --------
| /     | GET    | serves the main page
| /api/rooms | POST   | creates a new room and returns its name.
| /api/ws/rooms/:room | GET | upgrades to WebSocket and joins the room.


## Testing
To run all tests in the main package, run the following command from the root dir:
```
npm run test
```
