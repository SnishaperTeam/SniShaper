import React, { useEffect, useState } from 'react';
import { useTranslation } from '../i18n/I18nContext';
import { Globe, Link as LinkIcon, Users, Shield, Heart, RefreshCw, Download } from 'lucide-react';
import logoUrl from '../assets/logo.svg';
import { GetAppVersion, CheckUpdate, OpenURL } from '../api/bindings';
import Modal from '../components/Modal';
import { toast } from '../lib/toast';

interface UpdateResult {
  has_update: boolean;
  latest_version: string;
  download_url: string;
  message: string;
  error_detail?: string;
}

const About: React.FC = () => {
  const { t } = useTranslation();
  const [version, setVersion] = useState<string>('1.25');
  const [checkingUpdate, setCheckingUpdate] = useState<boolean>(false);
  const [showUpdateModal, setShowUpdateModal] = useState<boolean>(false);
  const [updateInfo, setUpdateInfo] = useState<{ latestVersion: string; downloadURL: string } | null>(null);

  useEffect(() => {
    // Get version from backend API
    GetAppVersion().then((v) => {
      if (v) {
        setVersion(v);
      }
    }).catch(() => {
      // Fallback to default version
      setVersion('1.25');
    });
  }, []);

  const handleOpenWebsite = () => {
    window.open('https://dongle.dpdns.org/', '_blank');
  };

  const handleDownloadUpdate = () => {
    if (updateInfo) {
      OpenURL(updateInfo.downloadURL);
      setShowUpdateModal(false);
    }
  };

  const handleCheckUpdate = async () => {
    if (checkingUpdate) return;
    
    console.log('[Update] Starting check...');
    setCheckingUpdate(true);
    try {
      console.log('[Update] Calling CheckUpdate API...');
      const result: UpdateResult = await CheckUpdate();
      console.log('[Update] Result:', result);
      
      switch (result.message) {
        case 'update_available':
          // Has new version - show modal
          setUpdateInfo({
            latestVersion: result.latest_version,
            downloadURL: result.download_url
          });
          setShowUpdateModal(true);
          break;
          
        case 'up_to_date':
          // Already latest - show toast
          toast.success(
            t('about.up_to_date'),
            t('about.up_to_date_desc').replace('{version}', version)
          );
          break;
          
        case 'dev_version':
          // Development version - show toast
          toast.info(
            t('about.dev_version'),
            t('about.dev_version_desc')
              .replace('{version}', version)
              .replace('{latestVersion}', result.latest_version)
          );
          break;
          
        case 'check_failed':
        default:
          // Check failed - show error toast with detailed message
          const errorKey = result.error_detail || 'check_failed';
          const errorMessage = t(`about.${errorKey}`) || t('about.check_failed_desc');
          toast.error(
            t('about.check_failed'),
            errorMessage
          );
          break;
      }
    } catch (error) {
      console.error('Check update error:', error);
      toast.error(
        t('about.check_failed'),
        t('about.check_failed_desc')
      );
    } finally {
      console.log('[Update] Resetting checking state');
      setCheckingUpdate(false);
    }
  };

  return (
    <div className="h-full flex flex-col overflow-y-auto">
      <div className="flex-1 p-8 max-w-4xl mx-auto w-full">
        {/* Header Section */}
        <div className="text-center mb-12">
          <div className="inline-flex items-center justify-center mb-6">
            <img
              src={logoUrl}
              alt="SniShaper logo"
              className="w-24 h-24 object-contain drop-shadow-[0_10px_30px_rgba(33,150,243,0.3)]"
            />
          </div>
          <h1 className="text-4xl font-black text-text-primary mb-3 tracking-tight">
            SniShaper
          </h1>
          <p className="text-lg text-text-secondary font-medium">
            {t('about.title')}
          </p>
          <div className="mt-4 inline-flex items-center gap-2 px-4 py-2 rounded-full bg-accent/10 border border-accent/20">
            <Shield size={16} className="text-accent" />
            <span className="text-sm font-bold text-accent">
              {t('about.version')}: {version}
            </span>
          </div>
        </div>

        {/* Description */}
        <div className="mb-10 p-6 rounded-2xl bg-background-card border border-border shadow-sm">
          <p className="text-text-secondary text-center leading-relaxed">
            {t('about.description')}
          </p>
        </div>

        {/* Info Cards Grid */}
        <div className="grid grid-cols-1 md:grid-cols-2 gap-6 mb-10">
          {/* Contributors Card */}
          <div className="p-6 rounded-2xl bg-background-card border border-border hover:border-accent/30 transition-all group">
            <div className="flex items-start gap-4">
              <div className="p-3 rounded-xl bg-success/10 text-success group-hover:bg-success/20 transition-colors">
                <Heart size={24} />
              </div>
              <div className="flex-1">
                <h3 className="text-sm font-bold text-text-secondary uppercase tracking-wider mb-2">
                  {t('about.contributors')}
                </h3>
                <p className="text-text-primary font-semibold text-lg">
                  mechrevo, dongzheyu, JetCPP-dongle
                </p>
              </div>
            </div>
          </div>

          {/* Maintainers Card */}
          <div className="p-6 rounded-2xl bg-background-card border border-border hover:border-accent/30 transition-all group">
            <div className="flex items-start gap-4">
              <div className="p-3 rounded-xl bg-warning/10 text-warning group-hover:bg-warning/20 transition-colors">
                <Shield size={24} />
              </div>
              <div className="flex-1">
                <h3 className="text-sm font-bold text-text-secondary uppercase tracking-wider mb-2">
                  {t('about.maintainers')}
                </h3>
                <p className="text-text-primary font-semibold text-lg">
                  JetCPP Team, Snishaper Team
                </p>
              </div>
            </div>
          </div>

          {/* Website Card */}
          <div 
            className="p-6 rounded-2xl bg-background-card border border-border hover:border-accent/30 transition-all group cursor-pointer"
            onClick={handleOpenWebsite}
          >
            <div className="flex items-start gap-4">
              <div className="p-3 rounded-xl bg-info/10 text-info group-hover:bg-info/20 transition-colors">
                <Globe size={24} />
              </div>
              <div className="flex-1">
                <h3 className="text-sm font-bold text-text-secondary uppercase tracking-wider mb-2">
                  {t('about.website')}
                </h3>
                <p className="text-accent font-semibold text-lg group-hover:underline">
                  dongle.dpdns.org
                </p>
              </div>
            </div>
          </div>
        </div>

        {/* Links Section */}
        <div className="flex flex-wrap justify-center gap-4">
          <button
            onClick={handleCheckUpdate}
            disabled={checkingUpdate}
            className="flex items-center gap-2 px-6 py-3 rounded-xl bg-success text-white font-bold hover:bg-success/90 transition-all shadow-lg shadow-success/25 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            <RefreshCw size={18} className={checkingUpdate ? 'animate-spin' : ''} />
            <span>{checkingUpdate ? t('about.checking') : t('about.check_update')}</span>
          </button>
          
          <button
            onClick={handleOpenWebsite}
            className="flex items-center gap-2 px-6 py-3 rounded-xl bg-accent text-white font-bold hover:bg-accent/90 transition-all shadow-lg shadow-accent/25"
          >
            <Globe size={18} />
            <span>{t('about.website')}</span>
          </button>
          
          <a
            href="https://github.com/SniShaper/SniShaper"
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center gap-2 px-6 py-3 rounded-xl bg-background-card border border-border text-text-primary font-bold hover:border-accent/50 hover:text-accent transition-all"
          >
            <LinkIcon size={18} />
            <span>GitHub</span>
          </a>
        </div>

        {/* Footer */}
        <div className="mt-16 text-center">
          <p className="text-text-secondary text-sm">
            © 2025-2026 SniShaper. All rights reserved.
          </p>
          <p className="text-text-muted text-xs mt-2">
            Made with ❤️ by the SniShaper community
          </p>
        </div>
      </div>

      {/* Update Modal */}
      <Modal
        isOpen={showUpdateModal}
        onClose={() => setShowUpdateModal(false)}
        title={updateInfo ? t('about.update_available') : t('common.status')}
      >
        <div className="space-y-4">
          {updateInfo ? (
            // Has update
            <>
              <div className="flex items-start gap-3 p-4 bg-success/10 border border-success/20 rounded-xl">
                <Download className="text-success shrink-0 mt-0.5" size={20} />
                <div>
                  <p className="text-sm font-bold text-text-primary mb-1">
                    {t('about.update_available')}
                  </p>
                  <p className="text-xs text-text-secondary">
                    {t('about.update_available_desc').replace('{version}', updateInfo.latestVersion)}
                  </p>
                </div>
              </div>
              <div className="flex gap-3">
                <button
                  onClick={handleDownloadUpdate}
                  className="flex-1 flex items-center justify-center gap-2 px-4 py-2.5 bg-success text-white rounded-xl font-bold hover:bg-success/90 transition-all"
                >
                  <Download size={16} />
                  <span>{t('common.confirm')}</span>
                </button>
                <button
                  onClick={() => setShowUpdateModal(false)}
                  className="flex-1 px-4 py-2.5 bg-background-hover text-text-primary rounded-xl font-bold hover:bg-background-card transition-all"
                >
                  {t('common.cancel')}
                </button>
              </div>
            </>
          ) : (
            // No update or error - determine from context
            <div className="text-center py-4">
              <p className="text-sm text-text-secondary">
                {t('about.up_to_date_desc').replace('{version}', version)}
              </p>
            </div>
          )}
        </div>
      </Modal>
    </div>
  );
};

export default About;
