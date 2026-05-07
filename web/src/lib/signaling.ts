import { RTC } from "./rtc";

// [TODO] don't hardcode this
const FALLBACK_HOSTS = ["localhost:8081", "localhost:8082", "localhost:8083"];

interface StartOpts {
  initialServer?: string;
  onLookupFailed?: () => void;
}

async function lookupRoom(room: string): Promise<string> {
  const seen = new Set<string>();
  const candidates = [location.host, ...FALLBACK_HOSTS].filter((h) => {
    if (seen.has(h)) return false;
    seen.add(h);
    return true;
  });
  for (const host of candidates) {
    try {
      const res = await fetch(
        `${location.protocol}//${host}/api/rooms/${room}`,
      );
      if (res.status === 404) {
        throw "not_found";
      }
      if (!res.ok) continue;
      return (await res.json()).server as string;
    } catch (e) {
      if (e === "not_found") throw e;
      // try next host
    }
  }
  throw "all_unreachable";
}

/**
 * Connect to the signaling server for a room and keep the connection
 * alive across signaling outages. On WS close, re-fetches the assigned
 * server and reconnects with exponential backoff.
 *
 * The RTC is NOT torn down on signaling drops — WebRTC peer connections
 * can survive without signaling. Cleanup (including rtc.close()) is the
 * caller's responsibility via the returned function.
 *
 * @returns A cleanup function. Call on unmount to stop reconnecting,
 *   close the WebSocket, and tear down the RTC.
 */
export function startSignaling(
  room: string,
  rtc: RTC,
  opts: StartOpts = {},
): () => void {
  let ws: WebSocket | null = null;
  let cancelled = false;
  let backoff = 500;
  let attempt = 0;

  async function connect() {
    let server = attempt === 0 ? opts.initialServer : undefined;
    if (!server) {
      try {
        server = await lookupRoom(room);
      } catch (e) {
        if (e === "not_found") {
          opts.onLookupFailed?.();
          return;
        }
        // "all_unreachable" or unexpected — back off and retry.
        scheduleReconnect();
        return;
      }
    }
    if (cancelled) return;

    ws = openSocket(room, rtc, server);
    ws.addEventListener("open", () => {
      console.log("WebSocket opened with ", server, ", resetting backoff");
      backoff = 500;
    });
    ws.addEventListener("close", () => {
      ws = null;
      if (cancelled) return;
      console.log("WebSocket closed with ", server, ", scheduling reconnect");
      scheduleReconnect();
    });
  }

  function scheduleReconnect() {
    if (cancelled) return;
    const delay = backoff;
    backoff = Math.min(backoff * 2, 10000);
    attempt++;
    setTimeout(() => {
      if (!cancelled) connect();
    }, delay);
  }

  connect();

  return () => {
    cancelled = true;
    ws?.close();
    rtc.close();
  };
}

function openSocket(room: string, rtc: RTC, server?: string): WebSocket {
  const proto = location.protocol === "https:" ? "wss" : "ws";
  const host = server || location.host;
  const ws = new WebSocket(`${proto}://${host}/api/ws/rooms/${room}`);
  rtc.setSendMethod((msg) => ws.send(msg));
  ws.onmessage = (e) => rtc.handleSignal(e.data);
  return ws;
}
