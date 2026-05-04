import { useNavigate } from "react-router-dom";

export default function Home() {
  const navigate = useNavigate();

  async function createRoom() {
    const res = await fetch("/api/rooms", { method: "POST" });
    const { room, server } = await res.json();
    // Pass `server` via router state so Room can skip the lookup on
    // the create flow. On refresh/direct-link, Room falls back to GET.
    navigate(`/rooms/${room}`, { state: { server } });
  }

  async function joinRoom(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const target = e.target as HTMLFormElement;
    const room = target.room.value;
    navigate(`/rooms/${room}`);
  }

  return (
    <main>
      <h1>YATA</h1>
      <button onClick={createRoom}>Create Room</button>
      <form onSubmit={joinRoom}>
        <input type="text" placeholder="Room ID" name="room" />
        <button type="submit">Join Room</button>
      </form>
    </main>
  );
}
