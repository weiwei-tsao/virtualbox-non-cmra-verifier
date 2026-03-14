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

  const exportCSV = async (filter?: MailboxFilter) => {
    try {
      showToast('Preparing your export...', 'info', 3000);

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

      // Extract filename from Content-Disposition header
      const contentDisposition = response.headers.get('Content-Disposition');
      let filename = 'mailboxes.csv';
      if (contentDisposition) {
        const match = contentDisposition.match(/filename=(.+)/);
        if (match && match[1]) {
          filename = match[1].replace(/"/g, '');
        }
      }

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
