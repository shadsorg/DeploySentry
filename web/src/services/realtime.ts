/**
 * Real-time data management for web dashboard
 * Provides SSE connections and periodic refresh capabilities
 */

export type RealtimeEventType =
  | 'refresh'
  | 'flag_updated'
  | 'deployment_status_changed'
  | 'release_promoted'
  | 'system_alert';

export interface RealtimeEvent {
  type: RealtimeEventType;
  timestamp: string;
  data?: Record<string, unknown>;
}

export type EventCallback = (event: RealtimeEvent) => void;

class RealtimeManager {
  private static instance: RealtimeManager;
  private eventSource: EventSource | null = null;
  private eventCallbacks = new Map<RealtimeEventType, Set<EventCallback>>();
  private refreshInterval: ReturnType<typeof setInterval> | null = null;
  private reconnectTimeout: ReturnType<typeof setTimeout> | null = null;
  private isConnected = false;
  private baseUrl = '';
  private reconnectAttempts = 0;
  private maxReconnectAttempts = 5;

  static getInstance(): RealtimeManager {
    if (!RealtimeManager.instance) {
      RealtimeManager.instance = new RealtimeManager();
    }
    return RealtimeManager.instance;
  }

  private constructor() {}

  /**
   * Initialize real-time updates
   */
  async initialize(
    options: {
      baseUrl?: string;
      apiKey?: string;
      refreshInterval?: number;
    } = {},
  ): Promise<void> {
    this.baseUrl = options.baseUrl || '';

    // Set up periodic refresh if specified
    if (options.refreshInterval) {
      this.refreshInterval = setInterval(() => {
        this.triggerRefresh();
      }, options.refreshInterval);
    }

    // Connect to SSE if URL provided
    if (this.baseUrl) {
      await this.connectSSE();
    }
  }

  /**
   * Connect to Server-Sent Events stream
   */
  private async connectSSE(): Promise<void> {
    try {
      const url = `${this.baseUrl}/api/v1/events/stream`;

      this.eventSource = new EventSource(url, {
        withCredentials: true,
      });

      this.eventSource.onopen = () => {
        console.log('[RealtimeManager] SSE connection established');
        this.isConnected = true;
        this.reconnectAttempts = 0;
        this.notifyConnectionChange();
      };

      this.eventSource.onmessage = (event) => {
        this.handleSSEData(event.data);
      };

      this.eventSource.onerror = (error) => {
        console.error('[RealtimeManager] SSE error:', error);
        this.handleSSEError();
      };
    } catch (error) {
      console.error('[RealtimeManager] Failed to connect SSE:', error);
      this.handleSSEError();
    }
  }

  /**
   * Handle incoming SSE data
   */
  private handleSSEData(data: string): void {
    try {
      const eventData = JSON.parse(data) as RealtimeEvent;
      this.dispatchEvent(eventData);
    } catch (error) {
      console.error('[RealtimeManager] Failed to parse SSE data:', error);
    }
  }

  /**
   * Handle SSE errors and attempt reconnection
   */
  private handleSSEError(): void {
    this.isConnected = false;
    this.notifyConnectionChange();

    if (this.eventSource) {
      this.eventSource.close();
      this.eventSource = null;
    }

    // Attempt to reconnect with exponential backoff
    if (this.reconnectAttempts < this.maxReconnectAttempts) {
      const delay = Math.min(1000 * Math.pow(2, this.reconnectAttempts), 30000);
      console.log(
        `[RealtimeManager] Reconnecting in ${delay}ms (attempt ${this.reconnectAttempts + 1})`,
      );

      this.reconnectTimeout = setTimeout(() => {
        this.reconnectAttempts++;
        this.connectSSE();
      }, delay);
    } else {
      console.error('[RealtimeManager] Max reconnection attempts reached');
    }
  }

  /**
   * Dispatch event to registered callbacks
   */
  private dispatchEvent(event: RealtimeEvent): void {
    const callbacks = this.eventCallbacks.get(event.type);
    if (callbacks) {
      callbacks.forEach((callback) => {
        try {
          callback(event);
        } catch (error) {
          console.error('[RealtimeManager] Error in event callback:', error);
        }
      });
    }

    // Also dispatch to 'all' listeners
    const allCallbacks = this.eventCallbacks.get('*' as RealtimeEventType);
    if (allCallbacks) {
      allCallbacks.forEach((callback) => {
        try {
          callback(event);
        } catch (error) {
          console.error('[RealtimeManager] Error in event callback:', error);
        }
      });
    }
  }

  /**
   * Trigger a refresh event for all subscribers
   */
  private triggerRefresh(): void {
    this.dispatchEvent({
      type: 'refresh',
      timestamp: new Date().toISOString(),
    });
  }

  /**
   * Manually trigger refresh
   */
  refresh(): void {
    this.triggerRefresh();
  }

  /**
   * Subscribe to specific event types
   */
  subscribe(eventTypes: RealtimeEventType[] | '*', callback: EventCallback): () => void {
    const types = eventTypes === '*' ? ['*' as RealtimeEventType] : eventTypes;

    types.forEach((type) => {
      if (!this.eventCallbacks.has(type)) {
        this.eventCallbacks.set(type, new Set());
      }
      this.eventCallbacks.get(type)!.add(callback);
    });

    // Return unsubscribe function
    return () => {
      types.forEach((type) => {
        const callbacks = this.eventCallbacks.get(type);
        if (callbacks) {
          callbacks.delete(callback);
          if (callbacks.size === 0) {
            this.eventCallbacks.delete(type);
          }
        }
      });
    };
  }

  /**
   * Check if connected to real-time updates
   */
  get connected(): boolean {
    return this.isConnected;
  }

  /**
   * Notify connection change listeners
   */
  private notifyConnectionChange(): void {
    this.dispatchEvent({
      type: 'system_alert',
      timestamp: new Date().toISOString(),
      data: {
        connected: this.isConnected,
        message: this.isConnected
          ? 'Real-time updates connected'
          : 'Real-time updates disconnected',
      },
    });
  }

  /**
   * Cleanup resources
   */
  dispose(): void {
    if (this.refreshInterval) {
      clearInterval(this.refreshInterval);
      this.refreshInterval = null;
    }

    if (this.reconnectTimeout) {
      clearTimeout(this.reconnectTimeout);
      this.reconnectTimeout = null;
    }

    if (this.eventSource) {
      this.eventSource.close();
      this.eventSource = null;
    }

    this.eventCallbacks.clear();
    this.isConnected = false;
  }
}

// React hook for using real-time updates
import { useEffect, useState, useCallback } from 'react';

export function useRealtimeUpdates(
  eventTypes: RealtimeEventType[] | '*' = ['refresh'],
  onEvent?: EventCallback,
): {
  connected: boolean;
  triggerRefresh: () => void;
} {
  const [connected, setConnected] = useState(false);
  const realtimeManager = RealtimeManager.getInstance();

  useEffect(() => {
    // Subscribe to connection status updates
    const unsubscribeStatus = realtimeManager.subscribe(['system_alert'], (event) => {
      if (event.data && typeof event.data === 'object' && 'connected' in event.data && typeof event.data.connected === 'boolean') {
        setConnected(event.data.connected);
      }
    });

    // Subscribe to requested events
    const unsubscribeEvents = onEvent ? realtimeManager.subscribe(eventTypes, onEvent) : null;

    // Set initial connection state
    setConnected(realtimeManager.connected);

    return () => {
      unsubscribeStatus();
      unsubscribeEvents?.();
    };
  }, [eventTypes, onEvent, realtimeManager]);

  const triggerRefresh = useCallback(() => {
    realtimeManager.refresh();
  }, [realtimeManager]);

  return { connected, triggerRefresh };
}

// Auto-refresh hook for data fetching
export function useAutoRefresh(
  fetchData: () => Promise<void>,
  options: {
    interval?: number;
    events?: RealtimeEventType[];
    enabled?: boolean;
  } = {},
): {
  refresh: () => Promise<void>;
  connected: boolean;
} {
  const {
    interval = 30000, // 30 seconds default
    events = ['refresh', 'flag_updated', 'deployment_status_changed', 'release_promoted'],
    enabled = true,
  } = options;

  const [isRefreshing, setIsRefreshing] = useState(false);

  const refresh = useCallback(async () => {
    if (isRefreshing) return;

    setIsRefreshing(true);
    try {
      await fetchData();
    } catch (error) {
      console.error('[useAutoRefresh] Error refreshing data:', error);
    } finally {
      setIsRefreshing(false);
    }
  }, [fetchData, isRefreshing]);

  const handleEvent = useCallback(
    (event: RealtimeEvent) => {
      if (!enabled) return;

      console.log('[useAutoRefresh] Received event:', event.type);
      refresh();
    },
    [refresh, enabled],
  );

  const { connected } = useRealtimeUpdates(events, handleEvent);

  // Set up interval-based refresh as fallback
  useEffect(() => {
    if (!enabled) return;

    const intervalId = setInterval(refresh, interval);
    return () => clearInterval(intervalId);
  }, [refresh, interval, enabled]);

  // Initial data fetch
  useEffect(() => {
    if (enabled) {
      refresh();
    }
  }, [enabled, refresh]); // Only run on mount and when enabled changes

  return { refresh, connected };
}

export default RealtimeManager;
