## YataType interface

All shared data types in the YATA framework share some common capabilities. In short,
all YATA types must be able to

* apply an update
* encode the current state vector
* encode the current state as an update

## YataList

The basic type is the YATA list, which supports the following operations,

* insert at an index
* delete at an index

Using the YATAList, we can easily implement other shared types, like Text and Array.

## Signaling server
We need, however, a central server to:
* keep track of the connectivity in a room
* relay SDP messages and ICE candidates

These connections are done via WebSocket. 

### Fault-tolerance
* For our MVP, SPOF: the room information lives in memory, managed by a lock. 
* In the future, we'll integrate with distributed in-memory like Redis.
