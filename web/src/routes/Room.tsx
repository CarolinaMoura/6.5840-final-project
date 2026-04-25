import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { connectSignaling } from '../lib/signaling'

export default function Room() {
  const { room } = useParams<{ room: string }>()
  const navigate = useNavigate()
  const [events, setEvents] = useState<string[]>([])

  useEffect(() => {
    if (!room) return
    const ws = connectSignaling(room, {
      onEvent: (msg) => setEvents((prev) => [...prev, msg]),
      onClose: () => setEvents((prev) => [...prev, 'disconnected']),
      onError: () => navigate('/'),
    })
    return () => ws.close()
  }, [room, navigate])

  return (
    <main>
      <h1>Room {room}</h1>
      <ul>
        {events.map((e, i) => (
          <li key={i}>{e}</li>
        ))}
      </ul>
    </main>
  )
}
