import React, { useEffect, useState } from 'react';
    import { api } from '../services/api';
    import { CrawlRun } from '../types';
    import { Play, RotateCw, AlertTriangle, CheckCircle, Clock } from 'lucide-react';
    
    export const Crawler: React.FC = () => {
      const [runs, setRuns] = useState<CrawlRun[]>([]);
      const [loading, setLoading] = useState(false);
      const [isStarting, setIsStarting] = useState(false);
    
      const refresh = async () => {
        setLoading(true);
        const data = await api.getCrawlRuns();
        setRuns(data);
        setLoading(false);
      };
    
      useEffect(() => {
        refresh();
      }, []);
    
      const handleRun = async () => {
        if (!window.confirm("Start a new scraping job? This usually takes 10-15 minutes.")) return;
        setIsStarting(true);
        try {
          await api.triggerCrawl();
          await refresh();
        } finally {
          setIsStarting(false);
        }
      };
    
      const getStatusColor = (status: string) => {
        switch (status) {
          case 'success': return 'bg-green-100 text-green-800';
          case 'failed': return 'bg-red-100 text-red-800';
          case 'running': return 'bg-blue-100 text-blue-800 animate-pulse';
          default: return 'bg-gray-100 text-gray-800';
        }
      };
    
      return (
        <div className="space-y-6">
          <div className="flex items-center justify-between">
            <div>
               <h1 className="text-2xl font-bold text-gray-900">Crawler Status</h1>
               <p className="text-sm text-gray-500">Manage background scraping jobs and Smarty validation.</p>
            </div>
            <button
              onClick={handleRun}
              disabled={isStarting}
              className="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-white bg-primary hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-primary disabled:opacity-50"
            >
              {isStarting ? <RotateCw className="animate-spin mr-2" size={16}/> : <Play className="mr-2" size={16}/>}
              Start New Job
            </button>
          </div>
    
          {/* Active Job Card */}
          {runs.length > 0 && runs[0].status === 'running' && (
            <div className="rounded-md bg-blue-50 p-4 border border-blue-200">
              <div className="flex">
                <div className="flex-shrink-0">
                  <RotateCw className="h-5 w-5 text-blue-400 animate-spin" aria-hidden="true" />
                </div>
                <div className="ml-3 flex-1 md:flex md:justify-between">
                  <p className="text-sm text-blue-700">
                    Job <strong>{runs[0].id}</strong> is currently running. Found {runs[0].totalFound} locations so far.
                  </p>
                  <p className="mt-3 text-sm md:mt-0 md:ml-6">
                    <span className="whitespace-nowrap font-medium text-blue-700 hover:text-blue-600 cursor-pointer" onClick={refresh}>
                      Refresh View <span aria-hidden="true">&rarr;</span>
                    </span>
                  </p>
                </div>
              </div>
            </div>
          )}
    
          {/* History List */}
          <div className="bg-white shadow overflow-hidden sm:rounded-md border border-gray-200">
            <div className="px-4 py-5 sm:px-6 border-b border-gray-200 flex justify-between items-center">
              <h3 className="text-lg leading-6 font-medium text-gray-900">Run History</h3>
              <button onClick={refresh} className="text-gray-400 hover:text-gray-600"><RotateCw size={16}/></button>
            </div>
            <ul role="list" className="divide-y divide-gray-200">
              {runs.map((run) => (
                <li key={run.id}>
                  <div className="px-4 py-4 sm:px-6">
                    <div className="flex items-center justify-between">
                      <p className="text-sm font-medium text-primary truncate">{run.id}</p>
                      <div className="ml-2 flex-shrink-0 flex">
                        <p className={`px-2 inline-flex text-xs leading-5 font-semibold rounded-full ${getStatusColor(run.status)}`}>
                          {run.status.toUpperCase()}
                        </p>
                      </div>
                    </div>
                    <div className="mt-2 sm:flex sm:justify-between">
                      <div className="sm:flex">
                        <p className="flex items-center text-sm text-gray-500 mr-6">
                          <CheckCircle className="flex-shrink-0 mr-1.5 h-4 w-4 text-green-400" />
                          Validated: {run.totalValidated}/{run.totalFound}
                        </p>
                        {run.totalFailed > 0 && (
                           <p className="flex items-center text-sm text-gray-500">
                           <AlertTriangle className="flex-shrink-0 mr-1.5 h-4 w-4 text-red-400" />
                           Failed: {run.totalFailed}
                         </p>
                        )}
                      </div>
                      <div className="mt-2 flex items-center text-sm text-gray-500 sm:mt-0">
                        <Clock className="flex-shrink-0 mr-1.5 h-4 w-4 text-gray-400" />
                        <p>
                          Started {new Date(run.startedAt).toLocaleString()}
                        </p>
                      </div>
                    </div>
                    {/* Error Logs Preview */}
                    {run.errorsSample.length > 0 && (
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
          </div>
        </div>
      );
    };