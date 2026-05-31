import { useState } from 'react';
import { trackEvent } from '@/services/analytics';
import {
  DaemonConnectionSection,
  AppPreferencesSection,
  ImportExportSection,
  DataRecoverySection,
  DangerZoneSection,
  ReplayToolSection,
  MLSettingsSection,
  ApiKeySection,
  UserProfileSection,
  ConnectedDevicesSection,
  CopyDiagnosticsSection,
} from '../components/settings/sections';
import { SettingsAccordion } from '../components/settings/SettingsAccordion';
import type { SettingsAccordionItem } from '../components/settings/SettingsAccordion';
import {
  useDaemonConnection,
  useLogReplay,
  useReplayTool,
  useDataManagement,
  useDeveloperMode,
  useSettings,
} from '../hooks';
import { uninstallDaemon } from '../services/api/system';
import './Settings.css';

const Settings = () => {
  // Local UI state
  const [saved, setSaved] = useState(false);

  // Settings from backend
  const {
    autoRefresh,
    refreshInterval,
    showNotifications,
    theme,
    // ML Settings
    mlEnabled,
    metaGoldfishEnabled,
    metaTop8Enabled,
    metaWeight,
    personalWeight,
    // ML Suggestion Preferences
    suggestionFrequency,
    minimumConfidence,
    showCardAdditions,
    showCardRemovals,
    showArchetypeChanges,
    learnFromMatches,
    learnFromDeckChanges,
    retentionDays,
    maxSuggestionsPerView,
    // Rotation settings
    rotationNotificationsEnabled,
    rotationNotificationThreshold,
    // State
    isLoading: isLoadingSettings,
    isSaving,
    error: settingsError,
    // Setters
    setAutoRefresh,
    setRefreshInterval,
    setShowNotifications,
    setTheme,
    setMLEnabled,
    setMetaGoldfishEnabled,
    setMetaTop8Enabled,
    setMetaWeight,
    setPersonalWeight,
    // ML Suggestion Preferences setters
    setSuggestionFrequency,
    setMinimumConfidence,
    setShowCardAdditions,
    setShowCardRemovals,
    setShowArchetypeChanges,
    setLearnFromMatches,
    setLearnFromDeckChanges,
    setRetentionDays,
    setMaxSuggestionsPerView,
    setRotationNotificationsEnabled,
    setRotationNotificationThreshold,
    saveSettings,
    resetToDefaults,
  } = useSettings();

  // Developer mode hook
  const { isDeveloperMode } = useDeveloperMode();

  // Custom hooks for state management
  const {
    connectionStatus,
  } = useDaemonConnection();

  const {
    clearDataBeforeReplay,
    setClearDataBeforeReplay,
    isReplaying,
    replayProgress,
    handleReplayLogs,
  } = useLogReplay();

  const {
    replayToolActive,
    replayToolPaused,
    replayToolProgress,
    replaySpeed,
    setReplaySpeed,
    replayFilter,
    setReplayFilter,
    pauseOnDraft,
    setPauseOnDraft,
    handleStartReplayTool,
    handlePauseReplayTool,
    handleResumeReplayTool,
    handleStopReplayTool,
  } = useReplayTool();

  const { handleExportData } = useDataManagement();

  // Derived state
  const isConnected = connectionStatus.status === 'connected';

  // Local handlers
  const handleSave = async () => {
    const success = await saveSettings();
    if (success) {
      setSaved(true);
      setTimeout(() => setSaved(false), 3000);
      trackEvent({
        name: 'feature_settings_changed',
        properties: { setting_section: 'preferences', setting_key: 'save' },
      });
    }
  };

  const handleReset = () => {
    resetToDefaults();
  };

  // Build accordion items - let React Compiler handle memoization
  const accordionItems: SettingsAccordionItem[] = (() => {
    const items: SettingsAccordionItem[] = [
      {
        id: 'user-profile',
        label: 'User Profile',
        icon: '👤',
        content: <UserProfileSection />,
      },
      {
        id: 'connection',
        label: 'Connection',
        icon: '🔌',
        content: (
          <DaemonConnectionSection
            connectionStatus={connectionStatus}
          />
        ),
      },
      {
        id: 'api-key',
        label: 'API Key',
        icon: '🔑',
        content: <ApiKeySection />,
      },
      {
        id: 'connected-devices',
        label: 'Connected Devices',
        icon: '🖥️',
        content: <ConnectedDevicesSection />,
      },
      {
        id: 'preferences',
        label: 'Preferences',
        icon: '⚙️',
        content: (
          <AppPreferencesSection
            autoRefresh={autoRefresh}
            onAutoRefreshChange={setAutoRefresh}
            refreshInterval={refreshInterval}
            onRefreshIntervalChange={setRefreshInterval}
            showNotifications={showNotifications}
            onShowNotificationsChange={setShowNotifications}
            theme={theme}
            onThemeChange={setTheme}
            rotationNotificationsEnabled={rotationNotificationsEnabled}
            onRotationNotificationsEnabledChange={setRotationNotificationsEnabled}
            rotationNotificationThreshold={rotationNotificationThreshold}
            onRotationNotificationThresholdChange={setRotationNotificationThreshold}
          />
        ),
      },
      {
        id: 'export',
        label: 'Export',
        icon: '📦',
        content: (
          <ImportExportSection onExportData={handleExportData} />
        ),
      },
      {
        id: 'data-recovery',
        label: 'Data Recovery',
        icon: '🔄',
        content: (
          <DataRecoverySection
            isConnected={isConnected}
            clearDataBeforeReplay={clearDataBeforeReplay}
            onClearDataBeforeReplayChange={setClearDataBeforeReplay}
            isReplaying={isReplaying}
            replayProgress={replayProgress}
            onReplayLogs={() => handleReplayLogs(isConnected)}
          />
        ),
      },
      {
        id: 'danger-zone',
        label: 'Danger Zone',
        icon: '⚠️',
        content: (
          <DangerZoneSection
            isConnected={isConnected}
            onUninstallDaemon={async (purge) => {
              const response = await uninstallDaemon({ purge });
              // Forward the backend's user-facing message (residual
              // platform-specific cleanup steps) to the section so it
              // can render it verbatim in the success panel.
              return response.message;
            }}
          />
        ),
      },
      {
        id: 'copy-diagnostics',
        label: 'Copy Diagnostics',
        icon: '📋',
        content: <CopyDiagnosticsSection />,
      },
      {
        id: 'ml-recommendations',
        label: 'ML / AI',
        icon: '🤖',
        content: (
          <MLSettingsSection
            mlEnabled={mlEnabled}
            onMLEnabledChange={setMLEnabled}
            metaGoldfishEnabled={metaGoldfishEnabled}
            onMetaGoldfishEnabledChange={setMetaGoldfishEnabled}
            metaTop8Enabled={metaTop8Enabled}
            onMetaTop8EnabledChange={setMetaTop8Enabled}
            metaWeight={metaWeight}
            onMetaWeightChange={setMetaWeight}
            personalWeight={personalWeight}
            onPersonalWeightChange={setPersonalWeight}
            // ML Suggestion Preferences
            suggestionFrequency={suggestionFrequency}
            onSuggestionFrequencyChange={setSuggestionFrequency}
            minimumConfidence={minimumConfidence}
            onMinimumConfidenceChange={setMinimumConfidence}
            showCardAdditions={showCardAdditions}
            onShowCardAdditionsChange={setShowCardAdditions}
            showCardRemovals={showCardRemovals}
            onShowCardRemovalsChange={setShowCardRemovals}
            showArchetypeChanges={showArchetypeChanges}
            onShowArchetypeChangesChange={setShowArchetypeChanges}
            learnFromMatches={learnFromMatches}
            onLearnFromMatchesChange={setLearnFromMatches}
            learnFromDeckChanges={learnFromDeckChanges}
            onLearnFromDeckChangesChange={setLearnFromDeckChanges}
            retentionDays={retentionDays}
            onRetentionDaysChange={setRetentionDays}
            maxSuggestionsPerView={maxSuggestionsPerView}
            onMaxSuggestionsPerViewChange={setMaxSuggestionsPerView}
          />
        ),
      },
    ];

    // Add Developer Tools section if developer mode is enabled
    if (isDeveloperMode) {
      items.push({
        id: 'developer-tools',
        label: 'Developer Tools',
        icon: '🛠️',
        content: (
          <ReplayToolSection
            isConnected={isConnected}
            replayToolActive={replayToolActive}
            replayToolPaused={replayToolPaused}
            replayToolProgress={replayToolProgress}
            replaySpeed={replaySpeed}
            onReplaySpeedChange={setReplaySpeed}
            replayFilter={replayFilter}
            onReplayFilterChange={setReplayFilter}
            pauseOnDraft={pauseOnDraft}
            onPauseOnDraftChange={setPauseOnDraft}
            onStartReplayTool={() => handleStartReplayTool(isConnected)}
            onPauseReplayTool={handlePauseReplayTool}
            onResumeReplayTool={handleResumeReplayTool}
            onStopReplayTool={handleStopReplayTool}
          />
        ),
      });
    }

    return items;
  })();

  return (
    <div className="page-container">
      <div className="settings-header">
        <h1 className="page-title">Settings</h1>
        {saved && <div className="save-notification">Settings saved successfully!</div>}
      </div>

      <div className="settings-content">
        <SettingsAccordion
          items={accordionItems}
          defaultExpanded={['connection']}
          allowMultiple={true}
        />

        {/* Settings Error */}
        {settingsError && (
          <div className="settings-error">Error: {settingsError}</div>
        )}

        {/* Action Buttons */}
        <div className="settings-actions">
          <button
            className="primary-button"
            onClick={handleSave}
            disabled={isSaving || isLoadingSettings}
          >
            {isSaving ? 'Saving...' : 'Save Settings'}
          </button>
          <button
            className="secondary-button"
            onClick={handleReset}
            disabled={isSaving || isLoadingSettings}
          >
            Reset to Defaults
          </button>
        </div>
      </div>

    </div>
  );
};

export default Settings;
