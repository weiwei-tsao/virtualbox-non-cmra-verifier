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
  status: 'running' | 'success' | 'failed' | 'partial_halt';
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
}
