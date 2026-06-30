import React from 'react';
import { cn } from '../../lib/utils';

interface ToggleProps {
  checked: boolean;
  onChange: (checked: boolean) => void;
  size?: 'sm' | 'md';
  disabled?: boolean;
}

const sizeClasses = {
  sm: { track: 'h-5 w-9', thumb: 'h-4 w-4', translate: 'translate-x-[18px]', gap: 'top-0.5 left-0.5' },
  md: { track: 'h-6 w-11', thumb: 'h-5 w-5', translate: 'translate-x-[22px]', gap: 'top-0.5 left-0.5' },
};

export const Toggle: React.FC<ToggleProps> = ({ checked, onChange, size = 'sm', disabled }) => {
  const s = sizeClasses[size];
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      disabled={disabled}
      onClick={() => onChange(!checked)}
      className={cn(
        s.track, 'rounded-full transition-all relative shrink-0',
        checked ? 'bg-accent' : 'bg-background-hover border border-border/40',
        disabled && 'opacity-50 cursor-not-allowed'
      )}
    >
      <div className={cn(
        'absolute rounded-full bg-white shadow-sm transition-transform duration-200',
        s.thumb, s.gap,
        checked ? `left-0 ${s.translate}` : 'left-0.5'
      )} />
    </button>
  );
};
