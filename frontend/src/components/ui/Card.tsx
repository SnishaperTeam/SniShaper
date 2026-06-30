import React from 'react';
import { cn } from '../../lib/utils';

interface CardProps {
  title?: string;
  icon?: React.ReactNode;
  children: React.ReactNode;
  className?: string;
  action?: React.ReactNode;
}

export const Card: React.FC<CardProps> = ({ title, icon, children, className, action }) => (
  <div className={cn(
    'p-6 bg-background-card border border-border rounded-2xl shadow-sm hover:shadow-md transition-all',
    className
  )}>
    {(title || icon) && (
      <div className="flex items-center gap-3 mb-4">
        {icon && <div className="text-accent">{icon}</div>}
        {title && <h3 className="text-[13px] font-bold text-text-secondary tracking-tight uppercase">{title}</h3>}
        {action && <div className="ml-auto">{action}</div>}
      </div>
    )}
    {children}
  </div>
);

interface SettingRowProps {
  title?: React.ReactNode;
  desc?: React.ReactNode;
  icon?: React.ReactNode;
  children: React.ReactNode;
}

export const SettingRow: React.FC<SettingRowProps> = ({ title, desc, icon, children }) => (
  <div className="flex items-start gap-5 p-5 bg-background-card border border-border rounded-xl hover:border-accent/40 transition-all group">
    <div className="flex flex-1 min-w-0 gap-4 items-center">
      {icon && (
        <div className="w-10 h-10 rounded-2xl bg-background-hover flex items-center justify-center text-text-secondary group-hover:text-accent transition-colors shrink-0">
          {icon}
        </div>
      )}
      <div className="min-w-0">
        {title && <h4 className="text-sm font-bold leading-snug">{title}</h4>}
        {desc && <p className="text-[11px] text-text-muted mt-0.5 leading-relaxed font-medium">{desc}</p>}
      </div>
    </div>
    <div className="shrink-0 self-center">{children}</div>
  </div>
);

interface StackedSettingRowProps {
  title: React.ReactNode;
  desc?: React.ReactNode;
  icon?: React.ReactNode;
  children: React.ReactNode;
}

export const StackedSettingRow: React.FC<StackedSettingRowProps> = ({ title, desc, icon, children }) => (
  <div className="p-5 bg-background-card border border-border rounded-xl hover:border-accent/40 transition-all group">
    <div className="flex items-center gap-4 min-w-0">
      {icon && (
        <div className="w-10 h-10 rounded-2xl bg-background-hover flex items-center justify-center text-text-secondary group-hover:text-accent transition-colors shrink-0">
          {icon}
        </div>
      )}
      <div className="min-w-0">
        {title && <h4 className="text-sm font-bold leading-snug">{title}</h4>}
        {desc && <p className="text-[11px] text-text-muted mt-0.5 leading-relaxed font-medium">{desc}</p>}
      </div>
    </div>
    <div className="mt-4">{children}</div>
  </div>
);

interface ServiceCardProps {
  title: string;
  icon: React.ReactNode;
  status: React.ReactNode;
  children: React.ReactNode;
  action?: React.ReactNode;
}

export const ServiceCard: React.FC<ServiceCardProps> = ({ title, icon, status, children, action }) => (
  <div className="bg-background-card border border-border rounded-2xl overflow-hidden shadow-sm flex flex-col hover:border-accent/30 transition-all">
    <div className="px-6 py-4 border-b border-border bg-background-soft/30 flex justify-between items-center shrink-0">
      <div className="flex items-center gap-3">
        <div className="text-accent">{icon}</div>
        <h3 className="text-sm font-black tracking-tight uppercase">{title}</h3>
      </div>
      <div className="flex items-center gap-3">
        {status}
        {action}
      </div>
    </div>
    <div className="p-6 flex-1 space-y-4">{children}</div>
  </div>
);

interface FeatureCardProps {
  icon: React.ReactNode;
  title: string;
  description: string;
  color: string;
}

export const FeatureCard: React.FC<FeatureCardProps> = ({ icon, title, description, color }) => (
  <div className={cn(
    'p-5 rounded-2xl bg-background-card border border-border transition-all duration-300 group hover:shadow-lg hover:-translate-y-0.5',
    `hover:border-${color}/30`
  )}>
    <div className={cn(
      'w-10 h-10 rounded-xl flex items-center justify-center mb-3 group-hover:scale-110 transition-transform',
      `bg-${color}/10 text-${color}`
    )}>
      {icon}
    </div>
    <h4 className="text-sm font-bold text-text-primary mb-1">{title}</h4>
    <p className="text-xs text-text-secondary leading-relaxed">{description}</p>
  </div>
);
