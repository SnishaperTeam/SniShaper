import React from 'react';
import { Minus, Square, X } from 'lucide-react';
import { 
  HandleWindowClose,
  WindowMinimise, 
  WindowToggleMaximise
} from "../api/bindings";

const WindowControls: React.FC = () => {
  const handleMinimise = async () => {
    try {
      await WindowMinimise();
    } catch (e) {
      console.error("WindowMinimise failed:", e);
    }
  };

  const handleToggleMaximise = async () => {
    try {
      await WindowToggleMaximise();
    } catch (e) {
      console.error("WindowToggleMaximise failed:", e);
    }
  };

  const handleClose = async () => {
    try {
      await HandleWindowClose();
    } catch (e) {
      console.error("HandleWindowClose failed:", e);
    }
  };

  return (
    <div className="flex items-center gap-1" style={{ "--wails-draggable": "no-drag" } as React.CSSProperties}>
      <button 
        onClick={handleMinimise}
        className="h-8 w-8 hover:bg-black/5 dark:hover:bg-white/10 rounded-md text-text-primary transition-colors flex items-center justify-center active:scale-95"
        title="最小化"
        type="button"
      >
        <Minus size={14} strokeWidth={2.5} />
      </button>
      <button 
        onClick={handleToggleMaximise}
        className="h-8 w-8 hover:bg-black/5 dark:hover:bg-white/10 rounded-md text-text-primary transition-colors flex items-center justify-center active:scale-95"
        title="最大化/还原"
        type="button"
      >
        <Square size={12} strokeWidth={3} />
      </button>
      <button 
        onClick={handleClose}
        className="h-8 w-8 hover:bg-danger/10 hover:text-danger rounded-md text-text-primary transition-colors flex items-center justify-center active:scale-95"
        title="关闭"
        type="button"
      >
        <X size={14} strokeWidth={2.5} />
      </button>
    </div>
  );
};

export default WindowControls;
