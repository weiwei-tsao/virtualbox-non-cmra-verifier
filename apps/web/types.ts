export interface StandardizedAddress {
  deliveryLine1: string;
  lastLine: string;
}

export interface Mailbox {
  id: string;
  name: string;
  street?: string;
  city?: string;
  state?: string;
  zip?: string;
  price: number;
  link: string;
  cmra?: 'Y' | 'N' | 'Unknown' | string;
  rdi?: 'Residential' | 'Commercial' | 'Unknown' | string;
  standardizedAddress?: StandardizedAddress;
  lastValidatedAt?: string;
  crawlRunId?: string;
  source?: 'ATMB' | 'iPost1' | string;
}

export interface CrawlRun {
  id: string;
  startedAt: string;
  finishedAt?: string;
  status: 'running' | 'success' | 'failed' | 'partial_halt' | 'timeout' | 'cancelled';
  stats: {
    found: number;
    validated: number;
    skipped: number;
    failed: number;
  };
  errorsSample?: Array<{ link: string; reason: string }>;
}

export interface MailboxFilter {
  state?: string;
  cmra?: 'Y' | 'N';
  rdi?: 'Residential' | 'Commercial';
  source?: 'ATMB' | 'iPost1';
  search?: string;
  page: number;
  pageSize: number;
}

export interface Stats {
  totalMailboxes: number;
  commercialCount: number;
  residentialCount: number;
  avgPrice: number;
  byState: { name: string; value: number }[];
  bySource: { name: string; value: number }[];
  lastUpdated?: string;
}

export interface ValidationStats {
  pending: number;
  validated: number;
  failed: number;
  needsRevalidation: number;
  retryScheduled: number;
  manualReview: number;
  total: number;
}

export interface ValidationRun {
  runId: string;
  status: 'running' | 'success' | 'partial' | 'failed';
  startedAt: string;
  finishedAt?: string;
  stats: {
    highPriorityProcessed: number;
    mediumPriorityProcessed: number;
    lowPriorityProcessed: number;
    succeeded: number;
    failed: number;
    quotaExhausted: boolean;
    quotaRemaining: number;
  };
  itemsSample?: Array<{
    mailboxId: string;
    name: string;
    address: string;
    status: string;
    error: string;
    cmra: string;
    rdi: string;
  }>;
  errorsSample?: Array<{ link: string; reason: string }>;
  triggerType: 'manual' | 'automatic' | 'revalidation';
}

export type ToastType = 'success' | 'error' | 'info' | 'warning';

export interface Toast {
  id: string;
  type: ToastType;
  message: string;
  duration?: number;
}

export interface ToastContextType {
  toasts: Toast[];
  showToast: (message: string, type: ToastType, duration?: number) => void;
  hideToast: (id: string) => void;
}
