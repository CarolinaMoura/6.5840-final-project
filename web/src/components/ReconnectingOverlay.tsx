import type { CSSProperties } from "react";

const BACKDROP_STYLE: CSSProperties = {
  position: "fixed",
  inset: 0,
  backgroundColor: "rgba(0, 0, 0, 0.5)",
  display: "flex",
  alignItems: "center",
  justifyContent: "center",
  zIndex: 1000,
};

const BOX_STYLE: CSSProperties = {
  backgroundColor: "white",
  padding: "2rem 3rem",
  borderRadius: "0.5rem",
  display: "flex",
  flexDirection: "column",
  alignItems: "center",
  gap: "1rem",
  boxShadow: "0 4px 20px rgba(0, 0, 0, 0.25)",
};

const SPINNER_STYLE: CSSProperties = {
  width: "2.5rem",
  height: "2.5rem",
  border: "4px solid #e0e0e0",
  borderTopColor: "#333",
  borderRadius: "50%",
  animation: "reconnecting-spin 1s linear infinite",
};

const LABEL_STYLE: CSSProperties = {
  fontSize: "1.125rem",
  fontWeight: 500,
};

export function ReconnectingOverlay() {
  return (
    <div style={BACKDROP_STYLE} role="alert" aria-live="polite">
      <div style={BOX_STYLE}>
        <div style={SPINNER_STYLE} />
        <div style={LABEL_STYLE}>Reconnecting</div>
      </div>
    </div>
  );
}
