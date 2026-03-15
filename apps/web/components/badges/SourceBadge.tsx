import React from 'react';

interface SourceBadgeProps {
  source: string;
}

export const SourceBadge: React.FC<SourceBadgeProps> = ({ source }) => {
  if (source === 'iPost1') {
    return (
      <span className="inline-flex items-center text-xs text-purple-600 font-medium bg-purple-50 px-2 py-0.5 rounded border border-purple-200">
        iPost1
      </span>
    );
  }
  return (
    <span className="inline-flex items-center text-xs text-indigo-600 font-medium bg-indigo-50 px-2 py-0.5 rounded border border-indigo-200">
      ATMB
    </span>
  );
};
