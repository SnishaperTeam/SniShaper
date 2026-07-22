import React, { useState, useEffect } from 'react';
import { Layers, Save } from '../lib/icons';
import { AddNAT64Profile, UpdateNAT64Profile } from '../api/bindings';
import { Button } from '../components/ui/Button';
import { useTranslation } from '../i18n/I18nContext';

interface NAT64ProfileFormProps {
  initialData?: any;
  onSuccess: () => void;
  onCancel: () => void;
}

const NAT64ProfileForm: React.FC<NAT64ProfileFormProps> = ({ initialData, onSuccess, onCancel }) => {
  const { t } = useTranslation();
  const [formData, setFormData] = useState<any>({
    id: '',
    name: '',
    prefix: ''
  });
  const [isSubmitting, setIsSubmitting] = useState(false);

  useEffect(() => {
    if (initialData) {
      setFormData({
        id: initialData.id || '',
        name: initialData.name || '',
        prefix: initialData.prefix || ''
      });
    }
  }, [initialData]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!formData.name.trim() || !formData.prefix.trim()) return;

    setIsSubmitting(true);
    try {
      if (formData.id) {
        await UpdateNAT64Profile(formData);
      } else {
        await AddNAT64Profile(formData);
      }
      onSuccess();
    } catch (err) {
      console.error(err);
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <form onSubmit={handleSubmit} className="space-y-6 text-text-primary">
      <div className="space-y-4 p-4 bg-accent/5 border border-accent/20 rounded-2xl">
        <div className="flex gap-3 items-center">
          <Layers className="text-accent" size={20} />
          <div>
            <h4 className="text-xs font-black uppercase tracking-widest text-accent">
              {formData.id ? t('proxies.edit_nat64') || '编辑 NAT64 配置' : t('proxies.add_nat64') || '添加 NAT64 配置'}
            </h4>
            <p className="text-[10px] text-accent/70 font-medium">
              {t('proxies.nat64_form_subtitle') || '配置独立映射规则前缀'}
            </p>
          </div>
        </div>
      </div>

      <div className="space-y-4">
        <div className="space-y-1.5">
          <label className="text-[10px] font-black text-text-secondary uppercase tracking-widest px-1">
            {t('proxies.nat64_form_name') || '配置名称'}
          </label>
          <input
            type="text"
            required
            value={formData.name}
            onChange={(e) => setFormData({ ...formData, name: e.target.value })}
            placeholder="例如：特定黑名单绕过"
            className="w-full bg-background-hover border border-border px-4 py-3 rounded-xl text-sm focus:ring-2 focus:ring-accent outline-none font-medium transition-all"
          />
        </div>

        <div className="space-y-1.5">
          <label className="text-[10px] font-black text-text-secondary uppercase tracking-widest px-1">
            {t('proxies.nat64_form_prefix') || 'NAT64 前缀'}
          </label>
          <input
            type="text"
            required
            value={formData.prefix}
            onChange={(e) => setFormData({ ...formData, prefix: e.target.value })}
            placeholder="例如：64:ff9b::"
            className="w-full bg-background-hover border border-border px-4 py-3 rounded-xl text-sm focus:ring-2 focus:ring-accent outline-none font-medium transition-all"
          />
        </div>
      </div>

      <div className="flex justify-end gap-3 pt-4 border-t border-border/40">
        <Button type="button" variant="secondary" onClick={onCancel} disabled={isSubmitting}>
          {t('common.cancel')}
        </Button>
        <Button type="submit" variant="primary" disabled={isSubmitting} className="flex items-center gap-1.5">
          <Save size={14} />
          {t('common.save')}
        </Button>
      </div>
    </form>
  );
};

export default NAT64ProfileForm;
