import React from 'react';
import { Building2, Home } from 'lucide-react';

interface RDIBadgeProps {
  rdi: string;
}

export const RDIBadge: React.FC<RDIBadgeProps> = ({ rdi }) => {
  if (rdi === 'Commercial') {
    return (
      <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-800">
        <Building2 size={12} className="mr-1" /> Commercial
      </span>
    );
  }
  return (
    <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800">
      <Home size={12} className="mr-1" /> Residential
    </span>
  );
};
