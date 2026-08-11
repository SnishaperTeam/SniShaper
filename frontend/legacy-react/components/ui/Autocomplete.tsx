import React, { useEffect, useMemo, useRef, useState } from 'react';
import {
  Box,
  TextField,
  Popper,
  Paper,
  IconButton,
  CircularProgress,
  Typography,
} from '@mui/material';
import { alpha } from '@mui/material/styles';
import { ChevronDown, X } from '../../lib/icons';
import { useClickOutside } from '../../lib/useClickOutside';
import { useTranslation } from '../../i18n/I18nContext';

/**
 * Autocomplete 输入组件
 *
 * 交互基准:MUI (Material-UI) Autocomplete。
 * 核心行为(修复"输入框在用户未主动操作时自动退出输入模式"的问题):
 *
 * 1. 输入框保持聚焦与输入状态,下拉列表随输入实时过滤;
 * 2. 仅当用户点击组件外部区域时才退出输入模式:
 *    - 收起下拉列表(onClose('blur'))
 *    - 输入框取消聚焦(onBlur 回调,便于父组件同步状态)
 * 3. 点击输入框自身、下拉选项、清除/展开按钮等组件内部元素不会退出;
 * 4. 通过 onMouseDown 阻止默认焦点转移,保证点击内部元素时输入框不失焦
 *    (移动端合成 mousedown 同样会进入 React 合成事件,preventDefault 依然生效);
 * 5. 桌面鼠标(mousedown)与移动触摸(touchstart)行为一致,
 *    外部点击检测由 useClickOutside 统一处理。
 */

export interface AutocompleteOption {
  label: string;
  value: string;
  [key: string]: unknown;
}

/** 与 MUI Autocomplete 对齐的关闭原因 */
export type AutocompleteCloseReason =
  | 'blur' // 点击组件外部区域
  | 'escape' // 按 Escape 键
  | 'selectOption' // 选中某个选项
  | 'clear' // 点击清除按钮
  | 'toggleInput'; // 点击展开/收起箭头

export interface AutocompleteProps<
  T extends AutocompleteOption = AutocompleteOption,
> {
  options: T[];
  value: T | null;
  onChange: (option: T | null) => void;
  onInputChange?: (input: string) => void;
  onOpen?: () => void;
  onClose?: (reason: AutocompleteCloseReason) => void;
  onBlur?: () => void;
  getOptionLabel?: (option: T) => string;
  filterOptions?: (options: T[], input: string) => T[];
  placeholder?: string;
  label?: string;
  helperText?: string;
  disabled?: boolean;
  loading?: boolean;
  clearable?: boolean;
  size?: 'small' | 'medium';
  noOptionsText?: string;
  className?: string;
  style?: React.CSSProperties;
}

const defaultFilter = <T extends AutocompleteOption>(
  options: T[],
  input: string,
): T[] => {
  const keyword = input.trim().toLowerCase();
  if (!keyword) return options;
  return options.filter((option) =>
    option.label.toLowerCase().includes(keyword),
  );
};

const defaultGetLabel = (option: AutocompleteOption) => option.label;

function Autocomplete<T extends AutocompleteOption>(props: AutocompleteProps<T>) {
  const {
    options,
    value,
    onChange,
    onInputChange,
    onOpen,
    onClose,
    onBlur,
    getOptionLabel = defaultGetLabel,
    filterOptions = defaultFilter,
    placeholder,
    label,
    helperText,
    disabled = false,
    loading = false,
    clearable = true,
    size = 'small',
    noOptionsText,
    className,
    style,
  } = props;

  const { t } = useTranslation();
  const resolvedNoOptionsText = noOptionsText ?? t('common.no_matches');

  const rootRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  // 下拉列表渲染在 Popper(portal)中,必须把 popupRef 一并交给外部点击检测,
  // 否则点击选项会被误判为"点击外部"
  const popupRef = useRef<HTMLDivElement>(null);

  const [open, setOpen] = useState(false);
  const [inputValue, setInputValue] = useState(() =>
    value ? getOptionLabel(value) : '',
  );
  const [highlightedIndex, setHighlightedIndex] = useState(-1);

  // 用 ref 持有回调与状态镜像,避免事件处理器读取到过期闭包
  const openRef = useRef(open);
  openRef.current = open;
  const onCloseRef = useRef(onClose);
  onCloseRef.current = onClose;
  const onOpenRef = useRef(onOpen);
  onOpenRef.current = onOpen;
  const onBlurRef = useRef(onBlur);
  onBlurRef.current = onBlur;
  const onInputChangeRef = useRef(onInputChange);
  onInputChangeRef.current = onInputChange;

  const filtered = useMemo(
    () => filterOptions(options, inputValue),
    [options, inputValue, filterOptions],
  );

  const listboxId = React.useId();

  // 下拉关闭时把内部输入同步回选中值(对应 MUI 的受控回填行为)
  useEffect(() => {
    if (!open) {
      setInputValue(value ? getOptionLabel(value) : '');
    }
  }, [open, value, getOptionLabel]);

  const openPopup = () => {
    if (openRef.current || disabled) return;
    setOpen(true);
    onOpenRef.current?.();
  };

  // 统一的关闭入口:仅当下拉处于打开状态时执行,保证 onClose 不重复触发
  const close = (reason: AutocompleteCloseReason) => {
    if (!openRef.current) return;
    setOpen(false);
    setHighlightedIndex(-1);
    onCloseRef.current?.(reason);
  };

  // 阻止 mousedown 的默认行为(焦点转移),保证点击内部元素时输入框保持聚焦
  const preventBlur = (event: React.MouseEvent<HTMLElement>) => {
    event.preventDefault();
  };

  const select = (option: T) => {
    onChange(option);
    setInputValue(getOptionLabel(option));
    close('selectOption');
    // 选中后输入框保持聚焦(MUI 行为)
    inputRef.current?.focus();
  };

  const handleClear = () => {
    onChange(null);
    setInputValue('');
    onInputChangeRef.current?.('');
    setHighlightedIndex(-1);
    openPopup(); // 清空后保持输入模式,不触发 onClose
    inputRef.current?.focus();
  };

  const handleInputChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    const text = event.target.value;
    setInputValue(text);
    onInputChangeRef.current?.(text);
    openPopup();
    setHighlightedIndex(-1);
  };

  const handleFocus = () => {
    openPopup();
  };

  const handleInputBlur = () => {
    onBlurRef.current?.();
  };

  const handleKeyDown = (event: React.KeyboardEvent<HTMLInputElement>) => {
    if (disabled) return;
    switch (event.key) {
      case 'ArrowDown':
        event.preventDefault();
        if (!openRef.current) openPopup();
        setHighlightedIndex((prev) =>
          filtered.length === 0 ? -1 : (prev + 1) % filtered.length,
        );
        break;
      case 'ArrowUp':
        event.preventDefault();
        if (!openRef.current) openPopup();
        setHighlightedIndex((prev) =>
          filtered.length === 0
            ? -1
            : (prev - 1 + filtered.length) % filtered.length,
        );
        break;
      case 'Enter':
        if (openRef.current && highlightedIndex >= 0 && filtered[highlightedIndex]) {
          event.preventDefault();
          select(filtered[highlightedIndex]);
        }
        break;
      case 'Escape':
        if (openRef.current) {
          event.preventDefault();
          close('escape'); // Escape 只收起下拉,输入框保持聚焦
        }
        break;
    }
  };

  const handleToggle = () => {
    if (openRef.current) {
      close('toggleInput');
    } else {
      openPopup();
    }
    inputRef.current?.focus();
  };

  // 核心:点击外部检测。
  // 仅当点击落在 rootRef / popupRef 之外时:收起下拉 + 取消聚焦 + 触发回调。
  // 捕获阶段原生事件保证:外部元素即使 stopPropagation,也不影响本次判断;
  // blur 尚未发生,此时关闭不会与 React 合成事件的时序互相干扰。
  useClickOutside(
    [rootRef, popupRef],
    () => {
      if (openRef.current) close('blur');
      if (document.activeElement === inputRef.current) {
        inputRef.current?.blur();
      }
    },
    !disabled,
  );

  return (
    <Box
      ref={rootRef}
      className={className}
      style={style}
      sx={{ position: 'relative', width: '100%' }}
    >
      <TextField
        fullWidth
        size={size}
        label={label}
        placeholder={placeholder}
        helperText={helperText}
        disabled={disabled}
        value={inputValue}
        onChange={handleInputChange}
        onFocus={handleFocus}
        onBlur={handleInputBlur}
        onKeyDown={handleKeyDown}
        slotProps={{
          htmlInput: {
            ref: inputRef,
            role: 'combobox',
            'aria-expanded': open,
            'aria-haspopup': 'listbox',
            'aria-controls': open ? listboxId : undefined,
            'aria-autocomplete': 'list',
          },
          input: {
            endAdornment: (
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.25 }}>
                {loading && <CircularProgress size={14} />}
                {clearable && value && (
                  <IconButton
                    size="small"
                    tabIndex={-1}
                    aria-label={t('common.clear')}
                    onMouseDown={preventBlur}
                    onClick={handleClear}
                    sx={{ p: 0.5 }}
                  >
                    <X size={14} />
                  </IconButton>
                )}
                <IconButton
                  size="small"
                  tabIndex={-1}
                  aria-label={t('common.expand_options')}
                  onMouseDown={preventBlur}
                  onClick={handleToggle}
                  sx={{
                    p: 0.5,
                    transition: 'transform 0.2s',
                    transform: open ? 'rotate(180deg)' : 'none',
                  }}
                >
                  <ChevronDown size={16} />
                </IconButton>
              </Box>
            ),
          },
        }}
      />
      <Popper
        open={open}
        anchorEl={inputRef.current}
        placement="bottom-start"
        style={{ zIndex: 1400, width: rootRef.current?.clientWidth }}
      >
        <Paper
          ref={popupRef}
          id={listboxId}
          role="listbox"
          elevation={3}
          sx={{
            mt: 0.5,
            maxHeight: 280,
            overflowY: 'auto',
            borderRadius: 2,
            border: 1,
            borderColor: 'divider',
          }}
        >
          {loading ? (
            <Box sx={{ display: 'flex', justifyContent: 'center', p: 2 }}>
              <CircularProgress size={20} />
            </Box>
          ) : filtered.length === 0 ? (
            <Typography
              variant="caption"
              sx={{ display: 'block', p: 1.5, color: 'text.secondary' }}
            >
              {resolvedNoOptionsText}
            </Typography>
          ) : (
            filtered.map((option, index) => {
              const selected = value?.value === option.value;
              const highlighted = index === highlightedIndex;
              return (
                <Box
                  key={option.value}
                  role="option"
                  aria-selected={selected}
                  onMouseDown={preventBlur}
                  onClick={() => select(option)}
                  sx={{
                    px: 1.5,
                    py: 1,
                    cursor: 'pointer',
                    typography: 'body2',
                    color: 'text.primary',
                    bgcolor: highlighted
                      ? (theme: any) => alpha(theme.palette.primary.main, 0.12)
                      : 'transparent',
                    '&:hover': {
                      bgcolor: (theme: any) =>
                        alpha(theme.palette.primary.main, 0.08),
                    },
                    ...(selected && { color: 'primary.main', fontWeight: 700 }),
                  }}
                >
                  {getOptionLabel(option)}
                </Box>
              );
            })
          )}
        </Paper>
      </Popper>
    </Box>
  );
}

export default Autocomplete;
