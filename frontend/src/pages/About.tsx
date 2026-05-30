import React, { useEffect, useState } from 'react';
import { useTranslation } from '../i18n/I18nContext';
import {
  Globe,
  Link as LinkIcon,
  Users,
  Shield,
  Heart,
  RefreshCw,
  Download,
  Sparkles,
  Zap,
  Lock,
  Code2,
  GitBranch,
  Megaphone,
  Map,
  ExternalLink
} from 'lucide-react';
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

const FeatureCard: React.FC<{
  icon: React.ReactNode;
  title: string;
  description: string;
  color: string;
}> = ({ icon, title, description, color }) => (
  <div className={`p-5 rounded-2xl bg-background-card border border-border hover:border-${color}/30 transition-all duration-300 group hover:shadow-lg hover:-translate-y-0.5`}>
    <div className={`w-10 h-10 rounded-xl bg-${color}/10 flex items-center justify-center mb-3 group-hover:scale-110 transition-transform`}>
      <div className={`text-${color}`}>{icon}</div>
    </div>
    <h4 className="text-sm font-bold text-text-primary mb-1">{title}</h4>
    <p className="text-xs text-text-secondary leading-relaxed">{description}</p>
  </div>
);

const About: React.FC = () => {
  const { t } = useTranslation();
  const [version, setVersion] = useState<string>('1.25');
  const [checkingUpdate, setCheckingUpdate] = useState<boolean>(false);
  const [showUpdateModal, setShowUpdateModal] = useState<boolean>(false);
  const [updateInfo, setUpdateInfo] = useState<{ latestVersion: string; downloadURL: string } | null>(null);

  useEffect(() => {
    GetAppVersion().then((v) => {
      if (v) {
        setVersion(v);
      }
    }).catch(() => {
      setVersion('1.25');
    });
  }, []);

  const handleOpenWebsite = () => {
    OpenURL('https://dongle.dpdns.org/');
  };

  const handleOpenGitHub = () => {
    OpenURL('https://github.com/SniShaper/SniShaper');
  };

  const handleOpenAdaptation = () => {
    OpenURL('https://github.com/SniShaper/SniShaper/issues/32');
  };

  const handleOpenDevPlan = () => {
    OpenURL('https://github.com/SniShaper/SniShaper/issues/36');
  };

  const handleDownloadUpdate = () => {
    if (updateInfo) {
      OpenURL(updateInfo.downloadURL);
      setShowUpdateModal(false);
    }
  };

  const handleCheckUpdate = async () => {
    if (checkingUpdate) return;
    
    setCheckingUpdate(true);
    try {
      const result: UpdateResult = await CheckUpdate();
      
      switch (result.message) {
        case 'update_available':
          setUpdateInfo({
            latestVersion: result.latest_version,
            downloadURL: result.download_url
          });
          setShowUpdateModal(true);
          break;
          
        case 'up_to_date':
          toast.success(
            t('about.up_to_date'),
            t('about.up_to_date_desc').replace('{version}', version)
          );
          break;
          
        case 'dev_version':
          toast.info(
            t('about.dev_version'),
            t('about.dev_version_desc')
              .replace('{version}', version)
              .replace('{latestVersion}', result.latest_version)
          );
          break;
          
        case 'check_failed':
        default:
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
      setCheckingUpdate(false);
    }
  };

  return (
    <div className="h-full flex flex-col overflow-y-auto">
      <div className="flex-1 p-8 max-w-5xl mx-auto w-full">
        {/* Hero Header */}
        <div className="relative mb-12 p-8 rounded-3xl bg-gradient-to-br from-accent/5 via-background-card to-accent/5 border border-border overflow-hidden">
          <div className="absolute inset-0 bg-[radial-gradient(ellipse_at_top_right,_var(--accent-soft)_0%,_transparent_50%)] opacity-50" />
          <div className="relative flex flex-col items-center text-center">
            <div className="relative mb-6">
              <div className="absolute inset-0 bg-accent/20 blur-2xl rounded-full" />
              <img
                src={logoUrl}
                alt="SniShaper logo"
                className="relative w-28 h-28 object-contain drop-shadow-[0_10px_30px_rgba(33,150,243,0.3)]"
              />
            </div>
            <h1 className="text-4xl font-black text-text-primary mb-2 tracking-tight">
              SniShaper
            </h1>
            <p className="text-lg text-text-secondary font-medium mb-4">
              {t('about.title')}
            </p>
            <div className="inline-flex items-center gap-2 px-5 py-2.5 rounded-full bg-accent/10 border border-accent/20 backdrop-blur-sm">
              <Shield size={16} className="text-accent" />
              <span className="text-sm font-bold text-accent">
                {t('about.version')}: {version}
              </span>
            </div>
          </div>
        </div>

        {/* Description */}
        <div className="mb-10 p-6 rounded-2xl bg-background-card border border-border">
          <p className="text-text-secondary text-center leading-relaxed text-[15px]">
            {t('about.description')}
          </p>
        </div>

        {/* Features Grid */}
        <div className="mb-10">
          <h2 className="text-lg font-bold text-text-primary mb-5 flex items-center gap-2">
            <Sparkles size={20} className="text-accent" />
            <span>{t('about.features')}</span>
          </h2>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <FeatureCard
              icon={<Lock size={20} />}
              title={t('about.feature_ech')}
              description={t('about.feature_ech_desc')}
              color="accent"
            />
            <FeatureCard
              icon={<Zap size={20} />}
              title={t('about.feature_fast')}
              description={t('about.feature_fast_desc')}
              color="success"
            />
            <FeatureCard
              icon={<Code2 size={20} />}
              title={t('about.feature_open')}
              description={t('about.feature_open_desc')}
              color="warning"
            />
          </div>
        </div>

        {/* Community Section */}
        <div className="mb-10">
          <h2 className="text-lg font-bold text-text-primary mb-5 flex items-center gap-2">
            <Heart size={20} className="text-danger" />
            <span>{t('about.community')}</span>
          </h2>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-5">
            {/* 网站适配征集 Card */}
            <div
              onClick={handleOpenAdaptation}
              className="p-6 rounded-2xl bg-gradient-to-br from-accent/5 to-background-card border border-border hover:border-accent/30 transition-all duration-300 group hover:shadow-lg hover:-translate-y-0.5 cursor-pointer"
            >
              <div className="flex items-start gap-4">
                <div className="p-3 rounded-xl bg-accent/10 text-accent group-hover:bg-accent/15 transition-colors">
                  <Megaphone size={24} />
                </div>
                <div className="flex-1">
                  <h3 className="text-base font-bold text-text-primary mb-1.5 flex items-center gap-2">
                    {t('about.site_adaptation')}
                    <ExternalLink size={14} className="text-text-muted opacity-0 group-hover:opacity-100 transition-opacity" />
                  </h3>
                  <p className="text-sm text-text-secondary mb-3 leading-relaxed">
                    {t('about.site_adaptation_desc')}
                  </p>
                  <span className="inline-flex items-center gap-1.5 text-sm font-bold text-accent group-hover:underline">
                    {t('about.participate_now')}
                    <ExternalLink size={14} />
                  </span>
                </div>
              </div>
            </div>

            {/* 开发计划 Card */}
            <div
              onClick={handleOpenDevPlan}
              className="p-6 rounded-2xl bg-gradient-to-br from-success/5 to-background-card border border-border hover:border-success/30 transition-all duration-300 group hover:shadow-lg hover:-translate-y-0.5 cursor-pointer"
            >
              <div className="flex items-start gap-4">
                <div className="p-3 rounded-xl bg-success/10 text-success group-hover:bg-success/15 transition-colors">
                  <Map size={24} />
                </div>
                <div className="flex-1">
                  <h3 className="text-base font-bold text-text-primary mb-1.5 flex items-center gap-2">
                    {t('about.development_plan')}
                    <ExternalLink size={14} className="text-text-muted opacity-0 group-hover:opacity-100 transition-opacity" />
                  </h3>
                  <p className="text-sm text-text-secondary mb-3 leading-relaxed">
                    {t('about.development_plan_desc')}
                  </p>
                  <span className="inline-flex items-center gap-1.5 text-sm font-bold text-success group-hover:underline">
                    {t('about.view_plan')}
                    <ExternalLink size={14} />
                  </span>
                </div>
              </div>
            </div>
          </div>
        </div>

        {/* Info Cards Grid */}
        <div className="grid grid-cols-1 md:grid-cols-2 gap-5 mb-10">
          {/* Contributors Card */}
          <div className="p-5 rounded-2xl bg-background-card border border-border hover:border-success/30 transition-all duration-300 group hover:shadow-lg">
            <div className="flex items-start gap-4">
              <div className="p-3 rounded-xl bg-success/10 text-success group-hover:bg-success/15 transition-colors">
                <Heart size={22} />
              </div>
              <div className="flex-1">
                <h3 className="text-xs font-bold text-text-muted uppercase tracking-wider mb-1.5">
                  {t('about.contributors')}
                </h3>
                <p className="text-text-primary font-semibold text-[15px] leading-snug">
                  mechrevo, dongzheyu, JetCPP-dongle
                </p>
              </div>
            </div>
          </div>

          {/* Maintainers Card */}
          <div className="p-5 rounded-2xl bg-background-card border border-border hover:border-warning/30 transition-all duration-300 group hover:shadow-lg">
            <div className="flex items-start gap-4">
              <div className="p-3 rounded-xl bg-warning/10 text-warning group-hover:bg-warning/15 transition-colors">
                <Users size={22} />
              </div>
              <div className="flex-1">
                <h3 className="text-xs font-bold text-text-muted uppercase tracking-wider mb-1.5">
                  {t('about.maintainers')}
                </h3>
                <p className="text-text-primary font-semibold text-[15px] leading-snug">
                  JetCPP Team, SniShaper Team
                </p>
              </div>
            </div>
          </div>

          {/* Website Card */}
          <div 
            className="p-5 rounded-2xl bg-background-card border border-border hover:border-accent/30 transition-all duration-300 group hover:shadow-lg cursor-pointer"
            onClick={handleOpenWebsite}
          >
            <div className="flex items-start gap-4">
              <div className="p-3 rounded-xl bg-accent/10 text-accent group-hover:bg-accent/15 transition-colors">
                <Globe size={22} />
              </div>
              <div className="flex-1">
                <h3 className="text-xs font-bold text-text-muted uppercase tracking-wider mb-1.5">
                  {t('about.website')}
                </h3>
                <p className="text-accent font-semibold text-[15px] group-hover:underline">
                  dongle.dpdns.org
                </p>
              </div>
            </div>
          </div>

          {/* GitHub Card */}
          <div
            onClick={handleOpenGitHub}
            className="p-5 rounded-2xl bg-background-card border border-border hover:border-text-primary/30 transition-all duration-300 group hover:shadow-lg cursor-pointer"
          >
            <div className="flex items-start gap-4">
              <div className="p-3 rounded-xl bg-text-primary/10 text-text-primary group-hover:bg-text-primary/15 transition-colors">
                <GitBranch size={22} />
              </div>
              <div className="flex-1">
                <h3 className="text-xs font-bold text-text-muted uppercase tracking-wider mb-1.5">
                  GitHub
                </h3>
                <p className="text-text-primary font-semibold text-[15px] group-hover:underline">
                  SniShaper/SniShaper
                </p>
              </div>
            </div>
          </div>
        </div>

        {/* Action Buttons */}
        <div className="flex flex-wrap justify-center gap-4 mb-12">
          <button
            onClick={handleCheckUpdate}
            disabled={checkingUpdate}
            className="flex items-center gap-2.5 px-6 py-3 rounded-xl bg-accent text-white font-bold hover:bg-accent-dim transition-all shadow-lg shadow-accent/25 disabled:opacity-50 disabled:cursor-not-allowed hover:shadow-xl hover:-translate-y-0.5 active:translate-y-0"
          >
            <RefreshCw size={18} className={checkingUpdate ? 'animate-spin' : ''} />
            <span>{checkingUpdate ? t('about.checking') : t('about.check_update')}</span>
          </button>
          
          <button
            onClick={handleOpenWebsite}
            className="flex items-center gap-2.5 px-6 py-3 rounded-xl bg-background-card border border-border text-text-primary font-bold hover:border-accent/50 hover:text-accent transition-all hover:shadow-lg hover:-translate-y-0.5 active:translate-y-0"
          >
            <Globe size={18} />
            <span>{t('about.website')}</span>
          </button>
          
          <button
            onClick={handleOpenGitHub}
            className="flex items-center gap-2.5 px-6 py-3 rounded-xl bg-background-card border border-border text-text-primary font-bold hover:border-text-primary/50 transition-all hover:shadow-lg hover:-translate-y-0.5 active:translate-y-0"
          >
            <LinkIcon size={18} />
            <span>GitHub</span>
          </button>
        </div>

        {/* Footer */}
        <div className="text-center pb-4">
          <p className="text-text-muted text-xs">
            © 2025-2026 SniShaper. {t('about.rights_reserved')}
          </p>
          <p className="text-text-muted/60 text-[11px] mt-2">
            {t('about.made_with')} ❤️ {t('about.by_community')}
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
            <>
              <div className="flex items-start gap-3 p-4 bg-accent/10 border border-accent/20 rounded-xl">
                <Download className="text-accent shrink-0 mt-0.5" size={20} />
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
                  className="flex-1 flex items-center justify-center gap-2 px-4 py-2.5 bg-accent text-white rounded-xl font-bold hover:bg-accent-dim transition-all"
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
