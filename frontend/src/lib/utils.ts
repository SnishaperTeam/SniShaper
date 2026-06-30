import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export function formatSpeed(bytes: number) {
  if (bytes < 1024) return `${Math.round(bytes)} B/s`;
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB/s`;
  return `${Math.round(bytes / (1024 * 1024))} MB/s`;
}

export function parseLatencyMs(latency: unknown) {
  if (typeof latency === 'number') return latency;
  if (typeof latency !== 'string') return 0;
  const match = latency.match(/^(\d+(?:\.\d+)?)\s*(ns|us|µs|ms|s)?$/i);
  if (!match) return 0;
  const value = parseFloat(match[1]);
  const unit = (match[2] || 'ms').toLowerCase();
  if (unit === 's') return value * 1000;
  if (unit === 'us' || unit === 'µs') return value / 1000;
  if (unit === 'ns') return value / 1000000;
  return value;
}

export function extractErrorMessage(err: any): string {
  if (!err) return '';
  if (typeof err === 'string') return err;
  if (typeof err?.message === 'string' && err.message.trim()) return err.message;
  if (typeof err?.cause === 'string' && err.cause.trim()) return err.cause;
  if (typeof err?.cause?.message === 'string' && err.cause.message.trim()) return err.cause.message;
  try { return JSON.stringify(err); }
  catch { return String(err); }
}

export function splitListInput(value: string) {
  return value.split(/[\n,;]+/).map(s => s.trim()).filter(Boolean);
}

export function joinListInput(items: string[] | undefined) {
  return (items || []).join('\n');
}
