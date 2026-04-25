import { useNavigate } from 'react-router-dom'

export default function Home() {
  const navigate = useNavigate()

  async function createRoom() {
    const res = await fetch('/api/rooms', { method: 'POST' })
    const { room } = await res.json()
    console.log(room)
    navigate(`/rooms/${room}`)
  }

  return (
    <main>
      <h1>YATA</h1>
      <button onClick={createRoom}>Create Room</button>
    </main>
  )
}
