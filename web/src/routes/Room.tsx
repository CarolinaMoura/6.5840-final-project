import { useEffect, useRef, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { connectSignaling } from "../lib/signaling";
import { RTC } from "../lib/rtc";

export default function Room() {
  const { room } = useParams<{ room: string }>();
  const navigate = useNavigate();
  const [events, setEvents] = useState<string[]>([]);
  const rtcRef = useRef<RTC | null>(null);
  if (rtcRef.current === null) {
    rtcRef.current = new RTC({
      onData: (peerId, data) => {
        console.log(data);
        setEvents((prev) => [...prev, `[${peerId}]: ${data}`]);
      },
      onPeersChanged: (peers) => {
        setEvents((prev) => [...prev, `Peers changed: ${peers.join(", ")}`]);
      },
    });
  }
  const rtc = rtcRef.current;

  useEffect(() => {
    if (!room) return;
    const ws = connectSignaling(room, rtc);

    return () => ws.close();
  }, [room, navigate]);

  return (
    <main>
      <button onClick={() => navigate("/")}> ⬅️ Jair & Carol</button>
      <h1>Room {room}</h1>
      <form
        onSubmit={(e) => {
          e.preventDefault();
          const target = document.getElementById("message") as HTMLInputElement;
          const msg = target.value;
          rtc.broadcast(msg);
          setEvents((prev) => [...prev, `[me]: ${msg}`]);
          target.value = "";
        }}
      >
        <input id="message" type="text" />
        <button type="submit">Send</button>
      </form>
      <ul>
        {events.map((e, i) => (
          <li key={i}>{e}</li>
        ))}
      </ul>
    </main>
  );
}
