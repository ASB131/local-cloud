import type { QueryClient } from "@tanstack/react-query";
import type { CloudWsEvent } from "@/api/types";

export type CloudWsClient = {
  close: () => void;
};

const INVALIDATE_PREFIXES = [
  ["folder", "children"],
  ["folder"],
  ["trash"],
  ["favorites"],
  ["search"],
  ["auth", "me"],
] as const;

function invalidateForEvent(qc: QueryClient, type: string) {
  if (
    type.startsWith("file.") ||
    type.startsWith("folder.") ||
    type === "favorite.changed" ||
    type === "trash.changed"
  ) {
    if (type.startsWith("file.")) {
      qc.invalidateQueries({ queryKey: ["file"] });
    }
    for (const key of INVALIDATE_PREFIXES) {
      qc.invalidateQueries({ queryKey: [...key] });
    }
  }
}

export function connectCloudWs(accessToken: string, qc: QueryClient): CloudWsClient {
  const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
  const url = `${proto}//${window.location.host}/api/v1/ws`;
  let ws: WebSocket | null = null;
  let closed = false;
  let retryMs = 1000;

  const connect = () => {
    if (closed) return;
    ws = new WebSocket(url);
    ws.onopen = () => {
      retryMs = 1000;
      ws?.send(JSON.stringify({ type: "auth", access_token: accessToken }));
    };
    ws.onmessage = (ev) => {
      try {
        const msg = JSON.parse(String(ev.data)) as CloudWsEvent;
        if (msg.type === "ping" || msg.type === "ready") return;
        invalidateForEvent(qc, msg.type);
      } catch {
        /* ignore */
      }
    };
    ws.onclose = () => {
      if (closed) return;
      window.setTimeout(connect, retryMs);
      retryMs = Math.min(retryMs * 2, 30_000);
    };
  };

  connect();

  return {
    close: () => {
      closed = true;
      ws?.close();
    },
  };
}
