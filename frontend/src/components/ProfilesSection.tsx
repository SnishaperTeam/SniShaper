import React, { useState, useEffect, useCallback, useRef } from 'react';
import {
  Box, Button, TextField, Dialog, DialogTitle, DialogContent, DialogActions,
  List, ListItem, ListItemText, Chip, Typography,
} from '@mui/material';
import { Plus, Edit, Copy, RefreshCw, Download, CheckCircle } from '../lib/icons';import { toast } from '../lib/toast';
import { useTranslation } from '../i18n/I18nContext';
import {
  ListProfiles, GetActiveProfile, CreateProfile, RenameProfile, DuplicateProfile,
  SwitchProfile, ExportProfile, ImportProfile, RevealProfileExport,
} from '../api/bindings';

// 配置文件管理：命名快照（settings + rules）的创建 / 重命名 / 复制 /
// 切换 / 导入 / 导出。切换会停止代理并断开活动连接，由确认对话框明确告知。
const ProfilesSection: React.FC = () => {
  const { t } = useTranslation();
  const [profiles, setProfiles] = useState<any[]>([]);
  const [active, setActive] = useState<string>('');
  const [busy, setBusy] = useState(false);
  // 切换确认
  const [switchTarget, setSwitchTarget] = useState<string | null>(null);
  // 名称输入对话框（create / rename / duplicate 共用）
  const [nameDialog, setNameDialog] = useState<{ mode: 'create' | 'rename' | 'duplicate'; source?: string } | null>(null);
  const [nameInput, setNameInput] = useState('');
  const fileInputRef = useRef<HTMLInputElement>(null);

  const reload = useCallback(async () => {
    try {
      const [list, act] = await Promise.all([ListProfiles(), GetActiveProfile()]);
      setProfiles(Array.isArray(list) ? list : []);
      setActive(typeof act === 'string' ? act : '');
    } catch { /* keep last known state */ }
  }, []);

  useEffect(() => { reload(); }, [reload]);

  const runAction = async (action: () => Promise<void>, successKey?: string) => {
    setBusy(true);
    try {
      await action();
      if (successKey) toast.success(t(successKey));
      await reload();
    } catch (e: any) {
      toast.error(t('profiles.action_failed'), String(e || ''));
    } finally {
      setBusy(false);
    }
  };

  const openNameDialog = (mode: 'create' | 'rename' | 'duplicate', source?: string) => {
    setNameDialog({ mode, source });
    setNameInput(mode === 'create' ? '' : (source || ''));
  };

  const submitNameDialog = () => {
    if (!nameDialog) return;
    const name = nameInput.trim();
    if (!name) return;
    const dialog = nameDialog;
    setNameDialog(null);
    if (dialog.mode === 'create') {
      runAction(async () => { await CreateProfile(name); }, 'profiles.created');
    } else if (dialog.mode === 'rename' && dialog.source) {
      runAction(async () => { await RenameProfile(dialog.source!, name); }, 'profiles.renamed');
    } else if (dialog.mode === 'duplicate' && dialog.source) {
      runAction(async () => { await DuplicateProfile(dialog.source!, name); }, 'profiles.duplicated');
    }
  };

  const confirmSwitch = async () => {
    const target = switchTarget;
    setSwitchTarget(null);
    if (!target) return;
    setBusy(true);
    try {
      await SwitchProfile(target);
      toast.success(t('profiles.switched'), target);
      // 后端已重载全部配置与端口，前端整体刷新以重建所有页面状态。
      window.location.reload();
    } catch (e: any) {
      toast.error(t('profiles.action_failed'), String(e || ''));
      setBusy(false);
    }
  };

  const handleImportFile = async (file: File | undefined) => {
    if (!file) return;
    setBusy(true);
    try {
      const content = await file.text();
      const result = await ImportProfile(content);
      if (result?.warnings?.length) {
        toast.info(t('profiles.imported_with_warnings'), result.warnings.join('; '));
      } else {
        toast.success(t('profiles.imported'), result?.name || '');
      }
      await reload();
    } catch (e: any) {
      toast.error(t('profiles.import_invalid'), String(e || ''));
    } finally {
      setBusy(false);
      if (fileInputRef.current) fileInputRef.current.value = '';
    }
  };

  const nameDialogLabels: Record<string, { title: string; action: string }> = {
    create: { title: 'profiles.create_title', action: 'profiles.create' },
    rename: { title: 'profiles.rename_title', action: 'profiles.rename' },
    duplicate: { title: 'profiles.duplicate_title', action: 'profiles.duplicate' },
  };

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <Typography variant="subtitle1" sx={{ fontWeight: 700 }}>{t('profiles.title')}</Typography>
        <Chip
          size="small"
          color={active ? 'success' : 'default'}
          variant={active ? 'filled' : 'outlined'}
          label={active ? `${t('profiles.active')}: ${active}` : t('profiles.none_active')}
        />
      </Box>
      <Typography variant="body2" color="text.secondary">{t('profiles.desc')}</Typography>

      {profiles.length > 0 && (
        <List dense disablePadding sx={{ border: '1px solid', borderColor: 'divider', borderRadius: 1.5 }}>
          {profiles.map((p) => (
            <ListItem
              key={p.name}
              secondaryAction={
                <Box sx={{ display: 'flex', gap: 0.5, alignItems: 'center' }}>
                  {p.name === active && <CheckCircle size={16} />}
                  <Button
                    size="small"
                    disabled={busy || p.name === active}
                    onClick={() => setSwitchTarget(p.name)}
                    startIcon={<RefreshCw size={14} />}
                  >
                    {t('profiles.switch')}
                  </Button>
                  <Button size="small" disabled={busy} onClick={() => openNameDialog('rename', p.name)} startIcon={<Edit size={14} />}>
                    {t('profiles.rename')}
                  </Button>
                  <Button size="small" disabled={busy} onClick={() => openNameDialog('duplicate', p.name)} startIcon={<Copy size={14} />}>
                    {t('profiles.duplicate')}
                  </Button>
                  <Button
                    size="small"
                    disabled={busy}
                    onClick={() => runAction(async () => {
                      const path = await ExportProfile(p.name);
                      RevealProfileExport(path as unknown as string);
                    }, 'profiles.exported')}
                    startIcon={<Download size={14} />}
                  >
                    {t('profiles.export')}
                  </Button>
                </Box>
              }
            >
              <ListItemText
                primary={p.name}
                secondary={p.updated_at ? `${t('profiles.updated')}: ${new Date(p.updated_at).toLocaleString()}` : undefined}
              />
            </ListItem>
          ))}
        </List>
      )}

      <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap' }}>
        <Button size="small" variant="outlined" disabled={busy} startIcon={<Plus size={16} />} onClick={() => openNameDialog('create')}>
          {t('profiles.create')}
        </Button>
        <Button size="small" variant="outlined" disabled={busy} onClick={() => fileInputRef.current?.click()}>
          {t('profiles.import')}
        </Button>
        <input
          ref={fileInputRef}
          type="file"
          accept="application/json,.json"
          style={{ display: 'none' }}
          onChange={(e) => handleImportFile(e.target.files?.[0])}
        />
      </Box>

      {/* 切换确认：明确说明安全行为 */}
      <Dialog open={!!switchTarget} onClose={() => setSwitchTarget(null)}>
        <DialogTitle>{t('profiles.switch_confirm_title')}</DialogTitle>
        <DialogContent>
          <Typography>{t('profiles.switch_confirm_desc', { name: switchTarget || '' })}</Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setSwitchTarget(null)}>{t('common.cancel')}</Button>
          <Button variant="contained" color="primary" onClick={confirmSwitch}>{t('profiles.switch')}</Button>
        </DialogActions>
      </Dialog>

      {/* 名称输入（创建 / 重命名 / 复制共用） */}
      <Dialog open={!!nameDialog} onClose={() => setNameDialog(null)}>
        <DialogTitle>{nameDialog ? t(nameDialogLabels[nameDialog.mode].title) : ''}</DialogTitle>
        <DialogContent>
          <TextField
            autoFocus
            fullWidth
            margin="dense"
            label={t('profiles.name_label')}
            value={nameInput}
            onChange={(e) => setNameInput(e.target.value)}
            onKeyDown={(e) => { if (e.key === 'Enter') submitNameDialog(); }}
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setNameDialog(null)}>{t('common.cancel')}</Button>
          <Button variant="contained" onClick={submitNameDialog} disabled={!nameInput.trim()}>
            {nameDialog ? t(nameDialogLabels[nameDialog.mode].action) : ''}
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
};

export default ProfilesSection;
