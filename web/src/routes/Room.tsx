import {
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type CSSProperties,
} from "react";
import { useLocation, useNavigate, useParams } from "react-router-dom";
import { startSignaling } from "../lib/signaling";
import { RTC } from "../lib/rtc";
import { TextPeer } from "../lib/textPeer";

/*
Each peer pushes its state vector to every other peer at most once per SYNC_INTERVAL_MS;
the receiver replies with the insert diff plus the current tombstone set.
*/
const SYNC_INTERVAL_MS = 200;

const TEXTAREA_STYLE: CSSProperties = {
  width: "100%",
  minHeight: "24rem",
  fontSize: "1.25rem",
  lineHeight: 1.5,
  padding: "1rem",
  fontFamily: "inherit",
  boxSizing: "border-box",
  resize: "vertical",
};

/**
 * The single-character edit a `beforeinput` event represents, after we've
 * dropped everything that isn't a single insert or single delete.
 */
type EditAction =
  | { kind: "insert"; index: number; char: string }
  | { kind: "delete"; index: number };

/**
 * Hashes an arbitrary string to a 32-bit signed integer, used to seed the
 * site identifier of a peer's YataText.
 */
async function hashStringToUUID(s: string): Promise<number> {
  const data = new TextEncoder().encode(s);
  const buf = await crypto.subtle.digest("SHA-256", data);
  return new DataView(buf).getInt32(0);
}

/**
 * Maps a native `beforeinput` event on a textarea to the single-character
 * edit it represents, or null if the input should be dropped (range edits,
 * paste, IME composition, multi-character inserts, etc.).
 */
function classifyEdit(
  event: InputEvent,
  el: HTMLTextAreaElement,
): EditAction | null {
  const start = el.selectionStart ?? el.value.length;
  const end = el.selectionEnd ?? start;
  if (start !== end) return null;

  switch (event.inputType) {
    case "deleteContentBackward":
      return start === 0 ? null : { kind: "delete", index: start - 1 };
    case "deleteContentForward":
      return start >= el.value.length ? null : { kind: "delete", index: start };
    case "insertText": {
      const char = event.data;
      return char !== null && char.length === 1
        ? { kind: "insert", index: start, char }
        : null;
    }
    case "insertLineBreak":
      return { kind: "insert", index: start, char: "\n" };
    default:
      return null;
  }
}

export default function Room() {
  const { room } = useParams<{ room: string }>();
  const navigate = useNavigate();
  const location = useLocation();
  const initialServer = (location.state as { server?: string } | null)?.server;

  const [text, setText] = useState("");
  const [ready, setReady] = useState(false);
  const [events, setEvents] = useState<string[]>([]);

  const peerRef = useRef<TextPeer | null>(null);
  const textareaRef = useRef<HTMLTextAreaElement | null>(null);
  const pendingCaretRef = useRef<number | null>(null);
  const rtcRef = useRef<RTC | null>(null);

  if (rtcRef.current === null) {
    rtcRef.current = new RTC({
      onData: (peerId, data) => {
        const peer = peerRef.current;
        const rtc = rtcRef.current;
        if (!peer || !rtc) return;
        const prevCaret = textareaRef.current?.selectionStart ?? null;
        peer.handleMessage(rtc, peerId, data);
        const next = peer.toString();
        if (prevCaret !== null) {
          pendingCaretRef.current = Math.min(prevCaret, next.length);
        }
        setText(next);
      },
      onPeerReady: (peerId) => {
        const peer = peerRef.current;
        const rtc = rtcRef.current;
        if (!peer || !rtc) return;
        peer.syncWithPeer(rtc, peerId);
      },
      onPeersChanged: (peers) => {
        setEvents((prev) => [...prev, `Peers changed: ${peers.join(", ")}`]);
      },
      onWelcome: (myId) => {
        if (peerRef.current) {
          setEvents((prev) => [...prev, `Reconnected`]);
          return;
        }
        hashStringToUUID(myId).then((uuid) => {
          peerRef.current = new TextPeer(uuid);
          setText(peerRef.current.toString());
          setReady(true);
          setEvents((prev) => [...prev, `Assigned id: ${uuid}`]);
        });
      },
    });
  }
  const rtc = rtcRef.current;

  useEffect(() => {
    if (!room) return;
    return startSignaling(room, rtc, {
      initialServer,
      onLookupFailed: () => navigate("/"),
      onStatus: (msg) => setEvents((prev) => [...prev, msg]),
    });
  }, [room, navigate, initialServer, rtc]);

  // restore the caret after react applies a new value from the peer's yataTextRef
  useLayoutEffect(() => {
    const pos = pendingCaretRef.current;
    if (pos !== null && textareaRef.current) {
      textareaRef.current.setSelectionRange(pos, pos);
      pendingCaretRef.current = null;
    }
  }, [text]);

  /*
  Periodically sync with other peers.
  */
  useEffect(() => {
    const id = setInterval(() => {
      const peer = peerRef.current;
      if (!peer) return;
      for (const peerId of rtc.peerIds) {
        peer.syncWithPeer(rtc, peerId);
      }
    }, SYNC_INTERVAL_MS);
    return () => clearInterval(id);
  }, [rtc]);

  // listen for beforeinput events on the textarea and apply the edit to the peer
  useEffect(() => {
    if (!ready) return;
    const el = textareaRef.current;
    if (!el) return;

    const onBeforeInput = (e: Event) => {
      const ev = e as InputEvent;
      ev.preventDefault();
      const peer = peerRef.current;
      if (!peer) return;

      const edit = classifyEdit(ev, el);
      if (!edit) return;

      if (edit.kind === "insert") {
        peer.insert(edit.index, edit.char);
        pendingCaretRef.current = edit.index + 1;
      } else {
        peer.delete(edit.index);
        pendingCaretRef.current = edit.index;
      }
      setText(peer.toString());
    };

    el.addEventListener("beforeinput", onBeforeInput);
    return () => el.removeEventListener("beforeinput", onBeforeInput);
  }, [ready]);

  return (
    <main>
      <button onClick={() => navigate("/")}> ⬅️ Jair & Carol</button>
      <h1>Room {room}</h1>
      <p>{ready ? "Connected" : "Connecting to signaling…"}</p>
      <textarea
        ref={textareaRef}
        value={text}
        disabled={!ready}
        placeholder={ready ? "Type!" : "Waiting for connection…"}
        rows={16}
        style={TEXTAREA_STYLE}
        onChange={() => {
          // native onChange events are ignored as the value is controlled by the peer
        }}
      />
      <ul>
        {events.map((e, i) => (
          <li key={i}>{e}</li>
        ))}
      </ul>
    </main>
  );
}
