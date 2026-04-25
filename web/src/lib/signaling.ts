type Handlers = {
  onEvent: (msg: string) => void
  onClose: () => void
  onError: () => void
}

export function connectSignaling(room: string, handlers: Handlers): WebSocket {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws';
  const ws = new WebSocket(`${proto}://${location.host}/api/ws/rooms/${room}`);
  ws.onmessage = (e) => handlers.onEvent(e.data);
  ws.onclose = handlers.onClose;
  ws.onerror = handlers.onError;
  return ws;
}
