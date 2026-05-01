import { useNavigate } from "react-router-dom";

export default function Home() {
  const navigate = useNavigate();

  async function createRoom() {
    const res = await fetch("/api/rooms", { method: "POST" });
    const { room } = await res.json();
    navigateToRoom(room);
  }

  async function joinRoom(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    console.log(e);
    const target = e.target as HTMLFormElement;
    const room = target.room.value;
    navigateToRoom(room);
  }

  async function navigateToRoom(room: string) {
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
