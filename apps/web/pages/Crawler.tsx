import React, { useState } from 'react';
import { useQuery, useQueryClient, useMutation } from '@tanstack/react-query';
import { api } from '../services/api';
import { Play, RotateCw, AlertTriangle, CheckCircle, Clock, XCircle, Shield } from 'lucide-react';

const DEFAULT_LINKS = (import.meta.env.VITE_CRAWL_LINKS || '')
  .split(',')
  .map(l => l.trim())
  .filter(Boolean);

export const Crawler: React.FC = () => {
  const queryClient = useQueryClient();
  const [isStartingCrawl, setIsStartingCrawl] = useState(false);
  const [isStartingValidation, setIsStartingValidation] = useState(false);

  const { data: runs = [], isLoading } = useQuery({
    queryKey: ['crawlRuns'],
    queryFn: api.getCrawlRuns,
    refetchInterval: (query) => {
      // Only poll every 5s when there's a running job
      const hasRunning = query.state.data?.some((r) => r.status === 'running');
      return hasRunning ? 5000 : false;
    },
  });

  const { data: validationStats } = useQuery({
    queryKey: ['validationStats'],
    queryFn: api.getValidationStats,
    refetchInterval: 10000, // Refresh every 10 seconds
  });

  const { data: validationRuns = [] } = useQuery({
    queryKey: ['validationRuns'],
    queryFn: api.getValidationRuns,
    refetchInterval: (query) => {
      const hasRunning = query.state.data?.some((r) => r.status === 'running');
      return hasRunning ? 5000 : false;
    },
  });

  const cancelMutation = useMutation({
    mutationFn: api.cancelCrawlRun,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['crawlRuns'] });
    },
  });

  const refresh = () => {
    queryClient.invalidateQueries({ queryKey: ['crawlRuns'] });
  };

  const handleCrawl = async () => {
    if (DEFAULT_LINKS.length === 0) {
      alert("Configure crawl links via VITE_CRAWL_LINKS (comma-separated) in your frontend env.");
      return;
    }
    if (!window.confirm("Start crawling to discover new addresses? This will mark them as pending validation.")) return;
    setIsStartingCrawl(true);
    try {
      await api.triggerCrawl(DEFAULT_LINKS);
      refresh();
    } catch (err) {
      console.error(err);
      alert("Failed to start crawl. Check console for details.");
    } finally {
      setIsStartingCrawl(false);
    }
  };

  const handleValidation = async () => {
    const confirmMsg = validationStats
      ? `Start validation for ${validationStats.pending + validationStats.needsRevalidation} pending addresses?`
      : "Start validation processing?";
    if (!window.confirm(confirmMsg)) return;
    setIsStartingValidation(true);
    try {
      const result = await api.triggerValidation();
      alert(result.message || "Validation started successfully");
      queryClient.invalidateQueries({ queryKey: ['validationStats'] });
    } catch (err) {
      console.error(err);
      alert("Failed to start validation. Check console for details.");
    } finally {
      setIsStartingValidation(false);
    }
  };

  const handleCancel = (runId: string) => {
    if (!window.confirm(`Cancel job ${runId}?`)) return;
    cancelMutation.mutate(runId);
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'success': return 'bg-green-100 text-green-800';
      case 'failed': return 'bg-red-100 text-red-800';
      case 'partial_halt': return 'bg-amber-100 text-amber-800';
      case 'running': return 'bg-blue-100 text-blue-800 animate-pulse';
      case 'timeout': return 'bg-orange-100 text-orange-800';
      case 'cancelled': return 'bg-gray-100 text-gray-800';
      default: return 'bg-gray-100 text-gray-800';
    }
  };

  const latestCrawlRun = runs[0];
  const latestValidationRun = validationRuns[0];

  const getValidationStatusColor = (status: string) => {
    switch (status) {
      case 'success': return 'bg-green-100 text-green-800';
      case 'partial': return 'bg-amber-100 text-amber-800';
      case 'failed': return 'bg-red-100 text-red-800';
      case 'running': return 'bg-blue-100 text-blue-800 animate-pulse';
      default: return 'bg-gray-100 text-gray-800';
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
           <h1 className="text-2xl font-bold text-gray-900">Jobs & Validation</h1>
           <p className="text-sm text-gray-500">Manage crawling jobs (discovery) and validation processing (Smarty API).</p>
        </div>
        <div className="flex gap-3">
          <button
            onClick={handleCrawl}
            disabled={isStartingCrawl}
            className="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-white bg-primary hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-primary disabled:opacity-50"
          >
            {isStartingCrawl ? <RotateCw className="animate-spin mr-2" size={16}/> : <Play className="mr-2" size={16}/>}
            Start Crawl
          </button>
          <button
            onClick={handleValidation}
            disabled={isStartingValidation}
            className="inline-flex items-center px-4 py-2 border border-gray-300 text-sm font-medium rounded-md shadow-sm text-gray-700 bg-white hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-primary disabled:opacity-50"
          >
            {isStartingValidation ? <RotateCw className="animate-spin mr-2" size={16}/> : <Shield className="mr-2" size={16}/>}
            Start Validation
          </button>
        </div>
      </div>

      {/* Active Crawl Job Card */}
      {latestCrawlRun && latestCrawlRun.status === 'running' && (
        <div className="rounded-md bg-blue-50 p-4 border border-blue-200">
          <div className="flex">
            <div className="flex-shrink-0">
              <RotateCw className="h-5 w-5 text-blue-400 animate-spin" aria-hidden="true" />
            </div>
            <div className="ml-3 flex-1 md:flex md:justify-between">
              <p className="text-sm text-blue-700">
                Crawl job <strong>{latestCrawlRun.id}</strong> is running. Found {latestCrawlRun.stats?.found ?? 0} locations so far.
              </p>
              <div className="mt-3 flex gap-4 md:mt-0 md:ml-6">
                <span className="whitespace-nowrap font-medium text-blue-700 hover:text-blue-600 cursor-pointer" onClick={refresh}>
                  Refresh View <span aria-hidden="true">&rarr;</span>
                </span>
                <span
                  className="whitespace-nowrap font-medium text-red-600 hover:text-red-500 cursor-pointer"
                  onClick={() => handleCancel(latestCrawlRun.id)}
                >
                  Cancel Job
                </span>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Active Validation Job Card */}
      {latestValidationRun && latestValidationRun.status === 'running' && (
        <div className="rounded-md bg-purple-50 p-4 border border-purple-200">
          <div className="flex">
            <div className="flex-shrink-0">
              <Shield className="h-5 w-5 text-purple-400 animate-pulse" aria-hidden="true" />
            </div>
            <div className="ml-3 flex-1">
              <p className="text-sm text-purple-700">
                Validation job <strong>{latestValidationRun.runId}</strong> is running.
                Processed {latestValidationRun.stats?.succeeded ?? 0} addresses so far.
              </p>
            </div>
          </div>
        </div>
      )}

      {/* Validation Queue Status Card */}
      {validationStats && (
        <div className="bg-white shadow rounded-lg border border-gray-200 p-6">
          <div className="flex items-center justify-between mb-4">
            <h3 className="text-lg font-medium text-gray-900">Validation Queue Status</h3>
            <Shield className="h-5 w-5 text-gray-400" />
          </div>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <div className="bg-yellow-50 rounded-lg p-3 border border-yellow-200">
              <div className="text-2xl font-bold text-yellow-800">{validationStats.pending}</div>
              <div className="text-xs text-yellow-600">Pending</div>
            </div>
            <div className="bg-orange-50 rounded-lg p-3 border border-orange-200">
              <div className="text-2xl font-bold text-orange-800">{validationStats.needsRevalidation}</div>
              <div className="text-xs text-orange-600">Needs Revalidation</div>
            </div>
            <div className="bg-green-50 rounded-lg p-3 border border-green-200">
              <div className="text-2xl font-bold text-green-800">{validationStats.validated}</div>
              <div className="text-xs text-green-600">Validated</div>
            </div>
            <div className="bg-red-50 rounded-lg p-3 border border-red-200">
              <div className="text-2xl font-bold text-red-800">{validationStats.failed}</div>
              <div className="text-xs text-red-600">Failed</div>
            </div>
          </div>
          <div className="mt-3 text-xs text-gray-500">
            Total: {validationStats.total} mailboxes •
            Retry Scheduled: {validationStats.retryScheduled} •
            Manual Review: {validationStats.manualReview}
          </div>
        </div>
      )}

      {/* Crawl Job History Section */}
      <div className="bg-white shadow overflow-hidden sm:rounded-md border border-gray-200">
        <div className="px-4 py-5 sm:px-6 border-b border-gray-200 flex justify-between items-center">
          <h3 className="text-lg leading-6 font-medium text-gray-900">Crawl Job History</h3>
          <button onClick={refresh} className="text-gray-400 hover:text-gray-600"><RotateCw size={16}/></button>
        </div>
        {isLoading && <div className="p-4 text-sm text-gray-500">Loading...</div>}
        {!isLoading && runs.length === 0 && <div className="p-4 text-sm text-gray-500">No crawl jobs yet. Click "Start Crawl" to discover new addresses.</div>}
        {!isLoading && runs.length > 0 && (
          <ul role="list" className="divide-y divide-gray-200">
            {runs.slice(0, 5).map((run) => (
              <li key={run.id}>
                <div className="px-4 py-4 sm:px-6">
                  <div className="flex items-center justify-between">
                    <p className="text-sm font-medium text-primary truncate">{run.id}</p>
                    <div className="ml-2 flex-shrink-0 flex items-center gap-2">
                      <p className={`px-2 inline-flex text-xs leading-5 font-semibold rounded-full ${getStatusColor(run.status)}`}>
                        {run.status.toUpperCase()}
                      </p>
                      {run.status === 'running' && (
                        <button
                          onClick={() => handleCancel(run.id)}
                          disabled={cancelMutation.isPending}
                          className="text-red-500 hover:text-red-700 disabled:opacity-50"
                          title="Cancel this job"
                        >
                          <XCircle size={16} />
                        </button>
                      )}
                    </div>
                  </div>
                  <div className="mt-2 sm:flex sm:justify-between">
                    <div className="sm:flex">
                      <p className="flex items-center text-sm text-gray-500 mr-6">
                        <CheckCircle className="flex-shrink-0 mr-1.5 h-4 w-4 text-green-400" />
                        Validated: {run.stats?.validated ?? 0}/{run.stats?.found ?? 0} (Skipped: {run.stats?.skipped ?? 0})
                      </p>
                      {run.stats?.failed ? (
                        <p className="flex items-center text-sm text-gray-500">
                          <AlertTriangle className="flex-shrink-0 mr-1.5 h-4 w-4 text-red-400" />
                          Failed: {run.stats.failed}
                        </p>
                      ) : null}
                    </div>
                    <div className="mt-2 flex items-center text-sm text-gray-500 sm:mt-0">
                      <Clock className="flex-shrink-0 mr-1.5 h-4 w-4 text-gray-400" />
                      <p>
                        Started {run.startedAt ? new Date(run.startedAt).toLocaleString() : '—'}
                      </p>
                    </div>
                  </div>
                  {run.errorsSample && run.errorsSample.length > 0 && (
                    <div className="mt-3 bg-red-50 p-2 rounded text-xs text-red-800 font-mono">
                      <p className="font-bold mb-1">Errors:</p>
                      {run.errorsSample.map((e, idx) => (
                        <div key={idx} className="truncate">[{e.reason}] {e.link}</div>
                      ))}
                    </div>
                  )}
                </div>
              </li>
            ))}
          </ul>
        )}
      </div>

      {/* Validation Job History Section */}
      <div className="bg-white shadow overflow-hidden sm:rounded-md border border-gray-200">
        <div className="px-4 py-5 sm:px-6 border-b border-gray-200 flex justify-between items-center">
          <h3 className="text-lg leading-6 font-medium text-gray-900">Validation Job History</h3>
          <button onClick={() => queryClient.invalidateQueries({ queryKey: ['validationRuns'] })} className="text-gray-400 hover:text-gray-600">
            <RotateCw size={16}/>
          </button>
        </div>
        {validationRuns.length === 0 && <div className="p-4 text-sm text-gray-500">No validation jobs yet. Click "Start Validation" to process pending addresses.</div>}
        {validationRuns.length > 0 && (
          <ul role="list" className="divide-y divide-gray-200">
            {validationRuns.slice(0, 5).map((run) => (
              <li key={run.runId}>
                <div className="px-4 py-4 sm:px-6">
                  <div className="flex items-center justify-between">
                    <p className="text-sm font-medium text-primary truncate">{run.runId}</p>
                    <div className="ml-2 flex-shrink-0 flex items-center gap-2">
                      {run.triggerType === 'manual' && (
                        <span className="px-2 inline-flex text-xs leading-5 font-semibold rounded-full bg-indigo-100 text-indigo-800">
                          MANUAL
                        </span>
                      )}
                      <p className={`px-2 inline-flex text-xs leading-5 font-semibold rounded-full ${getValidationStatusColor(run.status)}`}>
                        {run.status.toUpperCase()}
                      </p>
                    </div>
                  </div>
                  <div className="mt-2 sm:flex sm:justify-between">
                    <div className="sm:flex">
                      <p className="flex items-center text-sm text-gray-500 mr-6">
                        <CheckCircle className="flex-shrink-0 mr-1.5 h-4 w-4 text-green-400" />
                        Succeeded: {run.stats?.succeeded ?? 0}
                      </p>
                      {run.stats?.failed ? (
                        <p className="flex items-center text-sm text-gray-500 mr-6">
                          <AlertTriangle className="flex-shrink-0 mr-1.5 h-4 w-4 text-red-400" />
                          Failed: {run.stats.failed}
                        </p>
                      ) : null}
                      {run.stats?.quotaExhausted && (
                        <p className="flex items-center text-sm text-amber-600">
                          <AlertTriangle className="flex-shrink-0 mr-1.5 h-4 w-4" />
                          Quota Exhausted
                        </p>
                      )}
                    </div>
                    <div className="mt-2 flex items-center text-sm text-gray-500 sm:mt-0">
                      <Clock className="flex-shrink-0 mr-1.5 h-4 w-4 text-gray-400" />
                      <p>
                        Started {run.startedAt ? new Date(run.startedAt).toLocaleString() : '—'}
                      </p>
                    </div>
                  </div>
                  {run.errorsSample && run.errorsSample.length > 0 && (
                    <div className="mt-3 bg-red-50 p-2 rounded text-xs text-red-800 font-mono">
                      <p className="font-bold mb-1">Errors:</p>
                      {run.errorsSample.map((e, idx) => (
                        <div key={idx} className="truncate">[{e.reason}] {e.link}</div>
                      ))}
                    </div>
                  )}
                </div>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  );
};
