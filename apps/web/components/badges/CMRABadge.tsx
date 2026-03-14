import React from 'react';

interface CMRABadgeProps {
  cmra: string;
}

export const CMRABadge: React.FC<CMRABadgeProps> = ({ cmra }) => {
  if (cmra === 'Y') {
    return (
      <span className="inline-flex items-center text-xs text-amber-600 font-medium bg-amber-50 px-2 py-0.5 rounded border border-amber-200">
        CMRA
      </span>
    );
  }
  return (
    <span className="inline-flex items-center text-xs text-slate-500 font-medium bg-slate-100 px-2 py-0.5 rounded border border-slate-200">
      Not CMRA
    </span>
  );
};
