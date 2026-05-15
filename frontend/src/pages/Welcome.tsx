import React, { useState } from 'react';
import logoUrl from '../assets/logo.svg';
import { SetLanguage } from '../api/bindings';
import { useTranslation } from '../i18n/I18nContext';
import { Languages } from 'lucide-react';

interface WelcomeProps {
  onComplete: (lang: string) => void;
}

const Welcome: React.FC<WelcomeProps> = ({ onComplete }) => {
  const { setLanguage, t } = useTranslation();
  const [selected, setSelected] = useState<'zh' | 'en'>('zh');

  const handleStart = async () => {
    await SetLanguage(selected);
    localStorage.setItem('language', selected);
    setLanguage(selected);
    onComplete(selected);
  };

  return (
    <div className="fixed inset-0 z-[100] flex items-center justify-center bg-background p-6">
      <div className="w-full max-w-md space-y-8 text-center animate-in fade-in zoom-in duration-500">
        <div className="space-y-3">
          <div className="flex justify-center mb-6">
            <img src={logoUrl} className="h-20 w-20 drop-shadow-2xl animate-pulse" alt="logo" />
          </div>
          <h1 className="text-3xl font-bold text-text-primary tracking-tight">{t('welcome.title')}</h1>
          <p className="text-text-secondary">{t('welcome.subtitle')}</p>
        </div>

        <div className="grid grid-cols-2 gap-4">
          <button
            onClick={() => { setSelected('zh'); setLanguage('zh'); }}
            className={`flex flex-col items-center justify-center p-4 rounded-xl border-2 transition-all duration-200 ${
              selected === 'zh' 
                ? 'border-accent bg-accent/5 text-accent shadow-sm' 
                : 'border-border bg-background-soft/50 text-text-secondary hover:border-border-hover'
            }`}
          >
            <span className="text-lg font-medium mb-1 flex items-center gap-2">
              <Languages size={18} />
              简体中文
            </span>
            <span className="text-xs opacity-60">Chinese</span>
          </button>
          <button
            onClick={() => { setSelected('en'); setLanguage('en'); }}
            className={`flex flex-col items-center justify-center p-4 rounded-xl border-2 transition-all duration-200 ${
              selected === 'en' 
                ? 'border-accent bg-accent/5 text-accent shadow-sm' 
                : 'border-border bg-background-soft/50 text-text-secondary hover:border-border-hover'
            }`}
          >
            <span className="text-lg font-medium mb-1 flex items-center gap-2">
              <Languages size={18} />
              English
            </span>
            <span className="text-xs opacity-60">英语</span>
          </button>
        </div>

        <button
          onClick={handleStart}
          className="w-full py-3.5 bg-accent hover:bg-accent/90 text-white rounded-xl font-semibold shadow-lg shadow-accent/20 transition-all transform active:scale-[0.98]"
        >
          {t('welcome.start')}
        </button>
      </div>
    </div>
  );
};

export default Welcome;
