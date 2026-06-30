import React from 'react';
import { cn } from '../../lib/utils';

interface EmptyStateProps {
  icon: React.ReactNode;
  title: string;
  description?: string;
  action?: React.ReactNode;
  className?: string;
}

export const EmptyState: React.FC<EmptyStateProps> = ({ icon, title, description, action, className }) => (
  <div className={cn(
    'flex flex-col items-center justify-center py-16 text-text-muted opacity-50',
    className
  )}>
    <div className="mb-4 opacity-40">{icon}</div>
    <span className="text-xs font-black uppercase tracking-[0.2em]">{title}</span>
    {description && <p className="text-[11px] mt-2 text-center max-w-xs leading-relaxed">{description}</p>}
    {action && <div className="mt-4">{action}</div>}
  </div>
);
