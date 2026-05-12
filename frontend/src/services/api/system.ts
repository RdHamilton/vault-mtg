/**
 * System API service.
 * Replaces Wails system-related function bindings.
 */

import { get, post } from '../daemonClient';
import { gui, models } from '@/types/models';

// Re-export types for convenience
export type ConnectionStatus = gui.ConnectionStatus;

/**
 * Version information.
 */
export interface VersionInfo {
  version: string;
  service: string;
}

/**
 * Daemon status response.
 */
export interface DaemonStatus {
  status: string;
  connected: boolean;
}

/**
 * Database health information.
 */
export interface DatabaseHealth {
  status: string;
  lastWrite?: string;
}

/**
 * Log monitor health information.
 */
export interface LogMonitorHealth {
  status: string;
  lastRead?: string;
}

/**
 * WebSocket health information.
 */
export interface WebSocketHealth {
  status: string;
  connectedClients: number;
}

/**
 * Health metrics.
 */
export interface HealthMetrics {
  totalProcessed: number;
  totalErrors: number;
}

/**
 * System health status including backend sync timestamps.
 */
export interface HealthStatus {
  status: string;
  version: string;
  uptime: number;
  database: DatabaseHealth;
  logMonitor: LogMonitorHealth;
  websocket: WebSocketHealth;
  metrics: HealthMetrics;
}

/**
 * Get the current connection status.
 */
export async function getStatus(): Promise<ConnectionStatus> {
  return get<ConnectionStatus>('/system/status');
}

/**
 * Get the system health status including backend sync timestamps.
 */
export async function getHealth(): Promise<HealthStatus> {
  return get<HealthStatus>('/system/health');
}

/**
 * Get the daemon connection status.
 */
export async function getDaemonStatus(): Promise<DaemonStatus> {
  return get<DaemonStatus>('/system/daemon/status');
}

/**
 * Connect to the daemon.
 */
export async function connectDaemon(): Promise<{ status: string }> {
  return post<{ status: string }>('/system/daemon/connect');
}

/**
 * Disconnect from the daemon.
 */
export async function disconnectDaemon(): Promise<{ status: string }> {
  return post<{ status: string }>('/system/daemon/disconnect');
}

/**
 * Get the application version.
 */
export async function getVersion(): Promise<VersionInfo> {
  return get<VersionInfo>('/system/version');
}

/**
 * Get the database path.
 */
export async function getDatabasePath(): Promise<{ path: string }> {
  return get<{ path: string }>('/system/database/path');
}

/**
 * Set the database path.
 */
export async function setDatabasePath(path: string): Promise<{ status: string }> {
  return post<{ status: string }>('/system/database/path', { path });
}

/**
 * Get current account.
 */
export async function getCurrentAccount(): Promise<models.Account> {
  return get<models.Account>('/system/account');
}

/**
 * Daemon uninstall response shape.
 */
export interface UninstallResponse {
  status: string;
  message: string;
}

/**
 * Trigger a clean uninstall of the local daemon (Phase 2 PR #18).
 *
 * The daemon stops itself + removes the launchd plist / Task Scheduler
 * entry + (optionally) wipes its config directory. The daemon binary
 * itself stays on disk — the response message tells the user the
 * one remaining residual step (drag to Trash on macOS, Add/Remove
 * Programs on Windows). After the call returns the daemon will exit
 * within ~200ms.
 */
export async function uninstallDaemon(opts: { purge?: boolean } = {}): Promise<UninstallResponse> {
  const params = opts.purge ? '?purge=true' : '';
  return post<UninstallResponse>(`/system/uninstall${params}`);
}

/**
 * Export ML training data.
 */
export async function exportMLTrainingData(limit: number): Promise<gui.MLTrainingDataExport> {
  return get<gui.MLTrainingDataExport>(`/feedback/ml-training?limit=${limit}`);
}
