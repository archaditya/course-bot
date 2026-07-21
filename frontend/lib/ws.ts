// WebSocket singleton + client-side event bus.
// A single WS connection per session multiplexes:
//   - Course indexing status updates (patches React Query cache)
//   - Chat token streaming (forwarded to active chat window)
// See docs/11-frontend-architecture.md#websocket-layer.

type EventHandler = (payload: Record<string, unknown>) => void;

class EventBus {
  private handlers: Map<string, Set<EventHandler>> = new Map();

  on(event: string, handler: EventHandler) {
    if (!this.handlers.has(event)) this.handlers.set(event, new Set());
    this.handlers.get(event)!.add(handler);
    return () => this.off(event, handler);
  }

  off(event: string, handler: EventHandler) {
    this.handlers.get(event)?.delete(handler);
  }

  emit(event: string, payload: Record<string, unknown>) {
    this.handlers.get(event)?.forEach((h) => h(payload));
    // also emit wildcard listeners
    this.handlers.get('*')?.forEach((h) => h({ event, ...payload }));
  }
}

export const wsEvents = new EventBus();

let ws: WebSocket | null = null;
let reconnectTimer: ReturnType<typeof setTimeout> | null = null;

export function connectWebSocket(token: string): void {
  if (typeof window === 'undefined') return;
  if (ws?.readyState === WebSocket.OPEN) return;

  const WS_URL = process.env.NEXT_PUBLIC_WS_URL ?? 'ws://localhost:8080';
  ws = new WebSocket(`${WS_URL}/ws?token=${encodeURIComponent(token)}`);

  ws.onopen = () => {
    console.log('[ws] connected');
    if (reconnectTimer) clearTimeout(reconnectTimer);
  };

  ws.onmessage = (event) => {
    try {
      const msg = JSON.parse(event.data) as { event: string; payload: Record<string, unknown> };
      wsEvents.emit(msg.event, msg.payload);
    } catch {
      // non-JSON message — ignore
    }
  };

  ws.onclose = () => {
    console.log('[ws] disconnected, reconnecting in 3s...');
    reconnectTimer = setTimeout(() => connectWebSocket(token), 3000);
  };

  ws.onerror = (err) => {
    console.error('[ws] error', err);
  };
}

export function disconnectWebSocket(): void {
  if (reconnectTimer) clearTimeout(reconnectTimer);
  ws?.close();
  ws = null;
}
