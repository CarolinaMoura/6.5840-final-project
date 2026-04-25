// Manual testing:
//
// [Should create a room]
//     - in one terminal:
//         > go run .
//         > curl -X POST http://localhost:8080/api/rooms
//  Expected: {"room":"<room>"}
//
//
//  [Should upgrade connection to a websocket + should discover peers correctly]
//      - in one terminal:
//        > go run .
//        > curl -X POST http://localhost:8080/api/rooms
//      - (make sure you have websocat installed)
//        > websocat ws://localhost:8080/api/ws/rooms/<room>
//   Expected: `{"id":"<uuid1>","peers":<[]>,"type":"welcome"}`
//      - in another terminal:
//        > websocat ws://localhost:8080/api/ws/rooms/<room>
//   Expected: `{"id":"<uuid2>","peers":<[uuid1]>,"type":"welcome"}`
//      - in the first terminal:
//   Expected: `{"id":"<uuid2>","type":"peer-joined"}`

package main
