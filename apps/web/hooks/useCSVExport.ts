import { useToast } from '../contexts/ToastContext';
import { MailboxFilter } from '../types';

const API_BASE = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080';

const toQueryString = (params: Record<string, string | number | undefined>) =>
  Object.entries(params)
    .filter(([, v]) => v !== undefined && v !== '')
    .map(([k, v]) => `${encodeURIComponent(k)}=${encodeURIComponent(String(v))}`)
    .join('&');

export const useCSVExport = () => {
  const { showToast } = useToast();

  const generateFilename = (filter?: MailboxFilter): string => {
    const parts: string[] = ['mailbox'];

    // Add filter segments with user-friendly labels (in order: state, source, rdi, cmra)
    if (filter?.state) {
      parts.push(filter.state);
    }
    if (filter?.source) {
      parts.push(filter.source);
    }
    if (filter?.rdi) {
      parts.push(filter.rdi);
    }
    if (filter?.cmra === 'Y') {
      parts.push('CMRA');
    } else if (filter?.cmra === 'N') {
      parts.push('Non-CMRA');
    }

    // Add local timestamp in format: 2026_03_14_21_33_16
    const now = new Date();
    const year = now.getFullYear();
    const month = String(now.getMonth() + 1).padStart(2, '0');
    const day = String(now.getDate()).padStart(2, '0');
    const hours = String(now.getHours()).padStart(2, '0');
    const minutes = String(now.getMinutes()).padStart(2, '0');
    const seconds = String(now.getSeconds()).padStart(2, '0');
    const timestamp = `${year}_${month}_${day}_${hours}_${minutes}_${seconds}`;

    parts.push(timestamp);

    return parts.join('-') + '.csv';
  };

  const exportCSV = async (filter?: MailboxFilter) => {
    try {
      showToast('Preparing your export...', 'info', 8000);

      // Build query string
      const qs = filter ? toQueryString({
        state: filter.state,
        cmra: filter.cmra,
        rdi: filter.rdi,
        source: filter.source,
        active: 'true',
      }) : 'active=true';
      const url = `${API_BASE}/api/mailboxes/export?${qs}`;

      // Fetch CSV
      const response = await fetch(url);
      if (!response.ok) {
        throw new Error(`Export failed with status ${response.status}`);
      }

      // Generate user-friendly filename with local time
      const filename = generateFilename(filter);

      // Trigger download
      const blob = await response.blob();
      const downloadUrl = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = downloadUrl;
      a.download = filename;
      document.body.appendChild(a);
      a.click();
      window.URL.revokeObjectURL(downloadUrl);
      document.body.removeChild(a);

      showToast('CSV exported successfully!', 'success');
      return true;
    } catch (error) {
      const msg = error instanceof Error ? error.message : 'Export failed. Please try again.';
      showToast(msg, 'error');
      return false;
    }
  };

  return { exportCSV };
};
