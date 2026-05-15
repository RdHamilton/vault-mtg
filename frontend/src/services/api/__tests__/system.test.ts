import { describe, it, expect, vi, beforeEach } from 'vitest';
import * as system from '../system';

// Mock the daemonClient (system routes go to the local daemon)
vi.mock('../../daemonClient', () => ({
  get: vi.fn(),
  post: vi.fn(),
}));

// Mock runtimeContext so we can toggle desktop vs browser
vi.mock('@/lib/runtimeContext', () => ({
  isDesktopApp: vi.fn(),
}));

import { get, post } from '../../daemonClient';
import { isDesktopApp } from '@/lib/runtimeContext';

describe('system API', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('getStatus', () => {
    it('should call get with correct path', async () => {
      const mockStatus = { connected: true };
      vi.mocked(get).mockResolvedValue(mockStatus);

      const result = await system.getStatus();

      expect(get).toHaveBeenCalledWith('/system/status');
      expect(result).toEqual(mockStatus);
    });
  });

  describe('getHealth', () => {
    it('routes through daemon client (port 9001) in desktop context', async () => {
      vi.mocked(isDesktopApp).mockReturnValue(true);
      const mockHealth = {
        status: 'ok',
        version: '1.0.0',
        uptime: 3600,
        database: { status: 'ok', lastWrite: '2026-05-15T00:00:00Z' },
        logMonitor: { status: 'ok', lastRead: '2026-05-15T00:00:00Z' },
        websocket: { status: 'ok', connectedClients: 1 },
        metrics: { totalProcessed: 42, totalErrors: 0 },
      };
      vi.mocked(get).mockResolvedValue(mockHealth);

      const result = await system.getHealth();

      expect(isDesktopApp).toHaveBeenCalled();
      expect(get).toHaveBeenCalledWith('/system/health');
      expect(result).toEqual(mockHealth);
    });

    it('throws and does not call daemon client in non-desktop context', async () => {
      vi.mocked(isDesktopApp).mockReturnValue(false);

      await expect(system.getHealth()).rejects.toThrow(
        'getHealth() is only available in the desktop app context'
      );
      expect(get).not.toHaveBeenCalled();
    });
  });

  describe('getDaemonStatus', () => {
    it('should call get with correct path', async () => {
      const mockStatus = { status: 'running', connected: true };
      vi.mocked(get).mockResolvedValue(mockStatus);

      const result = await system.getDaemonStatus();

      expect(get).toHaveBeenCalledWith('/system/daemon/status');
      expect(result).toEqual(mockStatus);
    });
  });

  describe('connectDaemon', () => {
    it('should call post with correct path', async () => {
      const mockResult = { status: 'connected' };
      vi.mocked(post).mockResolvedValue(mockResult);

      const result = await system.connectDaemon();

      expect(post).toHaveBeenCalledWith('/system/daemon/connect');
      expect(result).toEqual(mockResult);
    });
  });

  describe('disconnectDaemon', () => {
    it('should call post with correct path', async () => {
      const mockResult = { status: 'disconnected' };
      vi.mocked(post).mockResolvedValue(mockResult);

      const result = await system.disconnectDaemon();

      expect(post).toHaveBeenCalledWith('/system/daemon/disconnect');
      expect(result).toEqual(mockResult);
    });
  });

  describe('getVersion', () => {
    it('should call get with correct path', async () => {
      const mockVersion = { version: '1.0.0', service: 'mtga-companion' };
      vi.mocked(get).mockResolvedValue(mockVersion);

      const result = await system.getVersion();

      expect(get).toHaveBeenCalledWith('/system/version');
      expect(result).toEqual(mockVersion);
    });
  });

  describe('getDatabasePath', () => {
    it('should call get with correct path', async () => {
      const mockPath = { path: '/path/to/db' };
      vi.mocked(get).mockResolvedValue(mockPath);

      const result = await system.getDatabasePath();

      expect(get).toHaveBeenCalledWith('/system/database/path');
      expect(result).toEqual(mockPath);
    });
  });

  describe('setDatabasePath', () => {
    it('should call post with path', async () => {
      const mockResult = { status: 'ok' };
      vi.mocked(post).mockResolvedValue(mockResult);

      const result = await system.setDatabasePath('/new/path');

      expect(post).toHaveBeenCalledWith('/system/database/path', { path: '/new/path' });
      expect(result).toEqual(mockResult);
    });
  });

  describe('getCurrentAccount', () => {
    it('should call get with correct path', async () => {
      const mockAccount = { id: 123, name: 'Player' };
      vi.mocked(get).mockResolvedValue(mockAccount);

      const result = await system.getCurrentAccount();

      expect(get).toHaveBeenCalledWith('/system/account');
      expect(result).toEqual(mockAccount);
    });
  });

  describe('exportMLTrainingData', () => {
    it('should call get with limit in query', async () => {
      const mockData = { records: [] };
      vi.mocked(get).mockResolvedValue(mockData);

      const result = await system.exportMLTrainingData(100);

      expect(get).toHaveBeenCalledWith('/feedback/ml-training?limit=100');
      expect(result).toEqual(mockData);
    });
  });

  describe('uninstallDaemon', () => {
    it('POSTs without purge by default', async () => {
      vi.mocked(post).mockResolvedValue({ status: 'scheduled', message: 'ok' });

      const result = await system.uninstallDaemon();

      expect(post).toHaveBeenCalledWith('/system/uninstall');
      expect(result).toEqual({ status: 'scheduled', message: 'ok' });
    });

    it('appends ?purge=true when opts.purge is true', async () => {
      vi.mocked(post).mockResolvedValue({ status: 'scheduled', message: 'ok' });

      await system.uninstallDaemon({ purge: true });

      expect(post).toHaveBeenCalledWith('/system/uninstall?purge=true');
    });

    it('omits the query string when opts.purge is false', async () => {
      vi.mocked(post).mockResolvedValue({ status: 'scheduled', message: 'ok' });

      await system.uninstallDaemon({ purge: false });

      expect(post).toHaveBeenCalledWith('/system/uninstall');
    });
  });
});
