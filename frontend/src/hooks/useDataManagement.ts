import { useCallback } from 'react';
import { matches } from '@/services/api';
import { downloadTextFile } from '@/utils/download';
import { showToast } from '../components/ToastContainer';

// Export functions
async function exportToJSON(): Promise<void> {
  const data = await matches.exportMatches('json');
  downloadTextFile(JSON.stringify(data, null, 2), 'mtga-matches.json');
}

async function exportToCSV(): Promise<void> {
  const data = await matches.exportMatches('csv');
  downloadTextFile(String(data), 'mtga-matches.csv');
}

export interface UseDataManagementReturn {
  /** Export match data to JSON or CSV */
  handleExportData: (format: 'json' | 'csv') => Promise<void>;
}

export function useDataManagement(): UseDataManagementReturn {
  const handleExportData = useCallback(async (format: 'json' | 'csv') => {
    try {
      if (format === 'json') {
        await exportToJSON();
      } else {
        await exportToCSV();
      }
      showToast.show(`Successfully exported data to ${format.toUpperCase()}!`, 'success');
    } catch (error) {
      showToast.show(`Failed to export data: ${error}`, 'error');
    }
  }, []);

  return {
    handleExportData,
  };
}
