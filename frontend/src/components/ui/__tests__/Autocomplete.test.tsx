import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import Autocomplete, { type AutocompleteOption } from '../Autocomplete';

/**
 * Autocomplete 核心逻辑测试
 *
 * 覆盖需求点:
 * - 输入过程中不自动退出输入模式
 * - 仅点击组件外部区域触发退出(关闭下拉 + 失焦 + onBlur/onClose 回调)
 * - 点击输入框自身 / 下拉选项 / 清除按钮等内部元素不退出
 * - 移动端 touchstart 与桌面端 mousedown 行为一致,且合成事件不重复触发
 * - 事件冒泡 / stopPropagation 不导致误退出(捕获阶段监听的优势)
 * - 键盘 Escape / 方向键 / Enter 行为
 * - 组件卸载后不再响应外部点击
 */

const options: AutocompleteOption[] = [
  { label: 'Alpha', value: 'alpha' },
  { label: 'Beta', value: 'beta' },
  { label: 'Gamma', value: 'gamma' },
];

function renderAutocomplete(overrides: Record<string, unknown> = {}) {
  const onOpen = vi.fn();
  const onClose = vi.fn();
  const onBlur = vi.fn();
  const onChange = vi.fn();
  const onInputChange = vi.fn();

  const utils = render(
    <Autocomplete
      options={options}
      value={null}
      onChange={onChange}
      onOpen={onOpen}
      onClose={onClose}
      onBlur={onBlur}
      onInputChange={onInputChange}
      placeholder="搜索"
      {...overrides}
    />,
  );

  return { onOpen, onClose, onBlur, onChange, onInputChange, ...utils };
}

const getInput = () => screen.getByRole('combobox');
const getListbox = () => screen.getByRole('listbox');

/** 打开下拉:点击输入框使其聚焦 */
const openPopup = async () => {
  await userEvent.click(getInput());
};

/** 模拟移动端触摸事件(jsdom 无 TouchEvent 构造,手动派发 Event) */
const fireTouchStartOutside = () => {
  fireEvent(
    document.body,
    new Event('touchstart', { bubbles: true, cancelable: true }),
  );
};

describe('Autocomplete 输入模式保持', () => {
  it('输入过程中下拉持续打开,不会自动退出', async () => {
    const { onClose } = renderAutocomplete();
    const input = getInput();
    await userEvent.click(input);
    await userEvent.type(input, 'be');

    expect(getListbox()).toBeInTheDocument();
    expect(input).toHaveFocus();
    expect(onClose).not.toHaveBeenCalled();
  });

  it('下拉打开后不做任何操作,输入模式保持', async () => {
    const { onClose } = renderAutocomplete();
    await openPopup();

    expect(getListbox()).toBeInTheDocument();
    expect(getInput()).toHaveFocus();
    expect(onClose).not.toHaveBeenCalled();
  });
});

describe('Autocomplete 点击外部退出', () => {
  it('点击组件外部空白区域:收起下拉、取消聚焦、触发 onClose 与 onBlur', async () => {
    const { onClose, onBlur } = renderAutocomplete();
    await openPopup();
    expect(getListbox()).toBeInTheDocument();

    await userEvent.click(document.body);

    expect(onClose).toHaveBeenCalledWith('blur');
    expect(onBlur).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
    expect(getInput()).not.toHaveFocus();
  });

  it('外部元素即使 stopPropagation,依然判定为外部点击(捕获阶段监听)', async () => {
    const { onClose } = renderAutocomplete();
    await openPopup();

    const outside = document.createElement('button');
    outside.textContent = '外部按钮';
    // 模拟外部元素阻止事件冒泡:捕获阶段在冒泡之前,不应影响判定
    outside.addEventListener('mousedown', (event) => event.stopPropagation());
    document.body.appendChild(outside);

    await userEvent.click(outside);

    expect(onClose).toHaveBeenCalledWith('blur');
    outside.remove();
  });

  it('组件卸载后点击外部不再触发回调', async () => {
    const { onClose, unmount } = renderAutocomplete();
    await openPopup();

    unmount();
    fireEvent.mouseDown(document.body);

    expect(onClose).not.toHaveBeenCalled();
  });
});

describe('Autocomplete 内部元素不退出', () => {
  it('点击输入框自身不会退出输入模式', async () => {
    const { onClose } = renderAutocomplete();
    const input = getInput();
    await userEvent.click(input);
    await userEvent.click(input);

    expect(onClose).not.toHaveBeenCalled();
    expect(getListbox()).toBeInTheDocument();
    expect(input).toHaveFocus();
  });

  it('点击下拉选项:选中并关闭,但不被误判为外部点击', async () => {
    const { onChange, onClose } = renderAutocomplete();
    await openPopup();

    await userEvent.click(screen.getByRole('option', { name: 'Beta' }));

    expect(onChange).toHaveBeenCalledWith({ label: 'Beta', value: 'beta' });
    expect(onClose).toHaveBeenCalledWith('selectOption');
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
    // 选中后输入框保持聚焦
    expect(getInput()).toHaveFocus();
  });

  it('点击清除按钮:清空选中值,但保持输入模式', async () => {
    const { onChange, onClose } = renderAutocomplete({ value: options[0] });
    await openPopup();

    await userEvent.click(screen.getByRole('button', { name: '清除' }));

    expect(onChange).toHaveBeenCalledWith(null);
    expect(onClose).not.toHaveBeenCalled();
    expect(getListbox()).toBeInTheDocument();
    expect(getInput()).toHaveFocus();
  });

  it('点击展开/收起箭头不会误判为外部点击', async () => {
    const { onClose } = renderAutocomplete();
    await openPopup();

    await userEvent.click(screen.getByRole('button', { name: '展开选项' }));

    expect(onClose).toHaveBeenCalledWith('toggleInput');
    expect(getInput()).toHaveFocus();
  });
});

describe('Autocomplete 移动端触摸行为', () => {
  it('触摸组件外部区域退出输入模式,且合成 mousedown 不重复触发', async () => {
    const { onClose } = renderAutocomplete();
    await openPopup();

    // 触摸外部 → 触发一次
    fireTouchStartOutside();
    expect(onClose).toHaveBeenCalledTimes(1);
    expect(onClose).toHaveBeenCalledWith('blur');

    // 浏览器在 touchstart 之后派发的合成 mousedown 应被去重跳过
    fireEvent.mouseDown(document.body);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('触摸输入框自身不退出输入模式', async () => {
    const { onClose } = renderAutocomplete();
    await openPopup();

    fireEvent.touchStart(getInput());
    fireEvent.touchStart(getInput());

    expect(onClose).not.toHaveBeenCalled();
    expect(getListbox()).toBeInTheDocument();
  });
});

describe('Autocomplete 键盘行为', () => {
  it('Escape 收起下拉但保持聚焦,不触发 onBlur', async () => {
    const { onClose, onBlur } = renderAutocomplete();
    await openPopup();

    await userEvent.keyboard('{Escape}');

    expect(onClose).toHaveBeenCalledWith('escape');
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
    expect(getInput()).toHaveFocus();
    expect(onBlur).not.toHaveBeenCalled();
  });

  it('ArrowDown 高亮 + Enter 选中选项', async () => {
    const { onChange } = renderAutocomplete();
    await openPopup();
    await userEvent.type(getInput(), 'be');
    await userEvent.keyboard('{ArrowDown}');
    await userEvent.keyboard('{Enter}');

    expect(onChange).toHaveBeenCalledWith({ label: 'Beta', value: 'beta' });
  });
});
