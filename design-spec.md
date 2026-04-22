## Signaling server
We need, however, a central server to:
* keep track of the connectivity in a room
* relay SDP messages and ICE candidates

These connections are done via WebSocket. 

### Fault-tolerance
* For our MVP, SPOF: the room information lives in memory, managed by a lock. 
* In the future, we'll integrate with distributed in-memory like Redis.