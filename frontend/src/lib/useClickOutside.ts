import { useEffect, useRef, type RefObject } from 'react';

/**
 * 点击外部区域检测 Hook
 * 行为基准:MUI Autocomplete 的 click-away 机制。
 *
 * 设计要点(对应"点击外部才退出输入模式"的需求):
 *
 * 1. 使用原生 document 捕获阶段监听 mousedown / touchstart,而非 React 合成事件:
 *    - blur 事件在 mousedown 之后才会由浏览器派发,因此在 mousedown 阶段判断
 *      点击位置,可以在失焦发生之前决定是否阻止焦点转移(即"保持输入模式")。
 *    - 捕获阶段在事件到达 React 根节点委托监听器之前执行,任何内部元素的
 *      stopPropagation 都无法拦截本次判断,避免事件冒泡导致的误判。
 *
 * 2. composedPath 判断点击目标是否位于任一容器节点内:
 *    - 覆盖普通 DOM 与 Shadow DOM 场景(事件被 retarget 后 target 可能不是真实节点)。
 *    - 输入框、下拉选项、按钮等"组件内部元素"都包含在容器 ref 内,不会触发回调。
 *
 * 3. 移动端去重:
 *    - 触摸触发 touchstart 之后,浏览器会紧接着派发一个合成 mousedown。
 *    - 通过 touched 标记,合成 mousedown 被跳过,保证一次外部触摸只回调一次。
 *      (与 MUI ClickAwayListener 的处理方式一致)
 *
 * 4. 回调与 ref 均通过 ref 持有最新值,effect 只依赖 active,
 *    监听器不会随组件每次渲染反复重建。
 */
export type ClickOutsideHandler = (event: MouseEvent | TouchEvent) => void;

type RefLike = RefObject<HTMLElement | null> | HTMLElement | null;

function resolveNode(ref: RefLike): HTMLElement | null {
  if (!ref) return null;
  return ref instanceof HTMLElement ? ref : ref.current;
}

export function useClickOutside(
  refs: Array<RefLike>,
  onClickOutside: ClickOutsideHandler,
  active = true,
) {
  const callbackRef = useRef(onClickOutside);
  callbackRef.current = onClickOutside;

  const refsRef = useRef(refs);
  refsRef.current = refs;

  useEffect(() => {
    if (!active) return;

    // 每次回调执行时解析最新 ref 对应的 DOM 节点
    const nodes = refsRef.current
      .map(resolveNode)
      .filter((node): node is HTMLElement => node !== null);

    // 判断事件目标是否落在任一容器内部
    const isInside = (event: Event): boolean => {
      const path =
        typeof event.composedPath === 'function'
          ? event.composedPath()
          : [event.target];
      return path.some(
        (element): boolean =>
          element instanceof HTMLElement && nodes.includes(element),
      );
    };

    // 移动端 touchstart 后浏览器会合成 mousedown,用于跳过重复回调
    let touched = false;

    const handleTouchStart = (event: TouchEvent) => {
      touched = true;
      if (!isInside(event)) callbackRef.current(event);
    };

    const handleMouseDown = (event: MouseEvent) => {
      // 跳过 touchstart 之后紧跟的合成 mousedown
      if (touched) {
        touched = false;
        return;
      }
      if (!isInside(event)) callbackRef.current(event);
    };

    document.addEventListener('touchstart', handleTouchStart, {
      capture: true,
      passive: true,
    });
    document.addEventListener('mousedown', handleMouseDown, { capture: true });

    return () => {
      document.removeEventListener('touchstart', handleTouchStart, {
        capture: true,
      });
      document.removeEventListener('mousedown', handleMouseDown, {
        capture: true,
      });
    };
  }, [active]);
}
