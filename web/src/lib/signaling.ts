import { RTC } from "./rtc";

/**
 * Connect to the signaling server for a given room and set up the RTC.
 * @param room The room id to connect to.
 * @param rtc The RTC instance to use.
 * @returns The WebSocket connection to the signaling server.
 */
export function connectSignaling(room: string, rtc: RTC): WebSocket {
  const proto = location.protocol === "https:" ? "wss" : "ws";
  const ws = new WebSocket(`${proto}://${location.host}/api/ws/rooms/${room}`);
  rtc.setSendMethod((msg) => ws.send(msg));
  ws.onmessage = (e) => rtc.handleSignal(e.data);
  ws.onclose = () => rtc.close();
  ws.onerror = () => rtc.close();
  return ws;
}
