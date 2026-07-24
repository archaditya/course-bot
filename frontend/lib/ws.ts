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


export function connectWebSocket(token: string): void {
  
}

export function disconnectWebSocket(): void {
  
}
