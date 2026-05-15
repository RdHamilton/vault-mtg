import { useState, useEffect, useCallback } from 'react';
import { useAuth } from '@clerk/react';
import { getDaemonHealth } from '@/services/api/bffHealth';
import { showToast } from '../components/ToastContainer';
import { gui } from '@/types/models';

// No-op functions - daemon control not implemented in REST API
// eslint-disable-next-line @typescript-eslint/no-unused-vars
const setDaemonPort = async (_port: number): Promise<void> => {
  // No-op in REST API mode
};

const reconnectToDaemon = async (): Promise<void> => {
  // No-op in REST API mode
};

const switchToStandaloneMode = async (): Promise<void> => {
  // No-op in REST API mode
};

const switchToDaemonMode = async (): Promise<void> => {
  // No-op in REST API mode
};

export interface UseDaemonConnectionReturn {
  /** Current connection status */
  connectionStatus: gui.ConnectionStatus;
  /** Current daemon mode setting */
  daemonMode: string;
  /** Current daemon port */
  daemonPort: number;
  /** Whether a reconnection is in progress */
  isReconnecting: boolean;
  /** Change the daemon port */
  handleDaemonPortChange: (port: number) => Promise<void>;
  /** Manually reconnect to daemon */
  handleReconnect: () => Promise<void>;
  /** Change connection mode */
  handleModeChange: (mode: string) => Promise<void>;
}

const defaultConnectionStatus = new gui.ConnectionStatus({
  status: 'standalone',
  connected: false,
  mode: 'standalone',
  url: 'ws://localhost:9999',
  port: 9999,
});

export function useDaemonConnection(): UseDaemonConnectionReturn {
  const { getToken, isSignedIn } = useAuth();
  const [connectionStatus, setConnectionStatus] = useState<gui.ConnectionStatus>(defaultConnectionStatus);
  const [daemonMode, setDaemonMode] = useState('auto');
  const [daemonPort, setDaemonPortState] = useState(9999);
  const [isReconnecting, setIsReconnecting] = useState(false);

  const loadConnectionStatus = useCallback(async () => {
    // Only probe daemon health in the desktop app context.
    // In browser/staging sessions window.__VAULTMTG_DESKTOP__ is unset,
    // so we skip the call entirely and return the default state to avoid
    // ERR_CONNECTION_REFUSED errors.
    if (!window.__VAULTMTG_DESKTOP__) {
      return;
    }

    if (!isSignedIn) {
      return;
    }

    try {
      const token = await getToken();
      if (!token) return;

      const result = await getDaemonHealth(token);
      const connected = result.status === 'connected';
      setConnectionStatus(
        gui.ConnectionStatus.createFrom({
          status: result.status,
          connected,
          mode: connected ? 'daemon' : 'standalone',
          url: 'ws://localhost:9999',
          port: 9999,
        }),
      );
    } catch {
      // Connection status load failed silently - UI will show default state
    }
  }, [getToken, isSignedIn]);

  useEffect(() => {
    loadConnectionStatus();
  }, [loadConnectionStatus]);

  const handleDaemonPortChange = useCallback(async (port: number) => {
    if (port < 1024 || port > 65535) {
      return;
    }

    setDaemonPortState(port);

    try {
      await setDaemonPort(port);
    } catch (error) {
      showToast.show(`Failed to set daemon port: ${error}`, 'error');
    }
  }, []);

  const handleReconnect = useCallback(async () => {
    setIsReconnecting(true);
    try {
      await reconnectToDaemon();
      await loadConnectionStatus();
      showToast.show('Successfully reconnected to daemon', 'success');
    } catch (error) {
      showToast.show(`Failed to reconnect to daemon: ${error}`, 'error');
    } finally {
      setIsReconnecting(false);
    }
  }, [loadConnectionStatus]);

  const handleModeChange = useCallback(async (mode: string) => {
    setDaemonMode(mode);

    try {
      if (mode === 'standalone') {
        await switchToStandaloneMode();
        await loadConnectionStatus();
        showToast.show('Switched to standalone mode', 'success');
      } else if (mode === 'daemon') {
        await switchToDaemonMode();
        await loadConnectionStatus();
        showToast.show('Switched to daemon mode', 'success');
      }
      // 'auto' mode is handled automatically by the app
    } catch (error) {
      showToast.show(`Failed to switch mode: ${error}`, 'error');
    }
  }, [loadConnectionStatus]);

  return {
    connectionStatus,
    daemonMode,
    daemonPort,
    isReconnecting,
    handleDaemonPortChange,
    handleReconnect,
    handleModeChange,
  };
}
