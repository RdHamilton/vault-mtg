import { SettingItem } from '../';

export interface ImportExportSectionProps {
  onExportData: (format: 'json' | 'csv') => void;
}

export function ImportExportSection({ onExportData }: ImportExportSectionProps) {
  return (
    <div className="settings-section">
      <h2 className="section-title">Export</h2>
      <div className="setting-description settings-section-description">
        Export your match history for backup or external analysis.
      </div>

      <SettingItem
        label="Export Data"
        description="Export your match history and statistics to a file for backup"
      >
        <button className="action-button" onClick={() => onExportData('json')}>
          Export to JSON
        </button>
        <button className="action-button" onClick={() => onExportData('csv')}>
          Export to CSV
        </button>
      </SettingItem>
    </div>
  );
}
