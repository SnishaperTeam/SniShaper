import React from 'react';
import { Loader2 } from '../../lib/icons';
import { cn } from '../../lib/utils';

interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'danger' | 'ghost' | 'outline' | 'success' | 'warning';
  size?: 'xs' | 'sm' | 'md' | 'lg';
  loading?: boolean;
  icon?: React.ReactNode;
}

const variantClasses = {
  primary: 'bg-accent text-white shadow-lg shadow-accent/20 hover:brightness-110',
  danger: 'bg-danger text-white shadow-lg shadow-danger/20 hover:brightness-110',
  ghost: 'bg-background-hover text-text-secondary hover:text-accent border border-border',
  outline: 'bg-background-card border border-border text-text-secondary hover:border-accent/40 hover:text-accent',
  success: 'bg-success text-white border-success/30 shadow-success/10',
  warning: 'bg-warning text-white border-warning/30 shadow-warning/10',
};

const sizeClasses = {
  xs: 'px-2.5 py-1.5 text-[10px]',
  sm: 'px-3 py-2 text-xs',
  md: 'px-4 py-2.5 text-xs',
  lg: 'px-6 py-3 text-sm',
};

export const Button: React.FC<ButtonProps> = ({
  variant = 'primary',
  size = 'md',
  loading,
  icon,
  children,
  className,
  disabled,
  ...props
}) => (
  <button
    className={cn(
      'inline-flex items-center justify-center gap-2 rounded-xl font-bold transition-all',
      'hover:scale-[1.02] active:scale-[0.98]',
      'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/50',
      variantClasses[variant],
      sizeClasses[size],
      (disabled || loading) && 'opacity-60 cursor-not-allowed hover:scale-100 active:scale-100',
      className
    )}
    disabled={disabled || loading}
    {...props}
  >
    {loading ? <Loader2 size={14} className="animate-spin" /> : icon}
    {children}
  </button>
);
