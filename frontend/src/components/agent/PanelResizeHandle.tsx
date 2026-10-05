import React, { useCallback, useRef } from 'react';
import { cn } from '../../lib/utils';

/**
 * Panel edge resize handle, ported from ZCode WorkspaceShellLayout's sidebar
 * separator: pointer-capture drag (no lost drags when the cursor leaves the
 * 1px strip), clamped widths, keyboard resize, and a grip line that only
 * appears on hover/active/focus so the chrome stays clean at rest.
 */

interface PanelResizeHandleProps {
  /** Which edge of the panel the handle sits on — determines the drag sign. */
  edge: 'left' | 'right';
  ariaLabel: string;
  /** Panel width at drag start. */
  getStartWidth: () => number;
  /** Called with the next clamped-or-unclamped width during drag/keyboard. */
  onResize: (nextWidth: number) => void;
  min: number;
  /** Max is a function so window resizes re-clamp naturally. */
  max: () => number;
  /** Keyboard step in px (ZCode: 16). */
  step?: number;
  className?: string;
}

export const PanelResizeHandle: React.FC<PanelResizeHandleProps> = ({
  edge,
  ariaLabel,
  getStartWidth,
  onResize,
  min,
  max,
  step = 16,
  className
}) => {
  const dragRef = useRef<{ pointerId: number; startX: number; startWidth: number } | null>(null);

  const clamp = useCallback(
    (width: number) => Math.round(Math.min(Math.max(width, min), max())),
    [min, max]
  );

  const handlePointerDown = useCallback(
    (event: React.PointerEvent<HTMLDivElement>) => {
      if (event.pointerType === 'mouse' && event.button !== 0) return;
      dragRef.current = {
        pointerId: event.pointerId,
        startX: event.clientX,
        startWidth: getStartWidth()
      };
      // Keep receiving moves even when the cursor leaves the thin strip.
      event.currentTarget.setPointerCapture(event.pointerId);
      event.preventDefault();
    },
    [getStartWidth]
  );

  const handlePointerMove = useCallback(
    (event: React.PointerEvent<HTMLDivElement>) => {
      const drag = dragRef.current;
      if (!drag || drag.pointerId !== event.pointerId) return;
      event.preventDefault();
      const delta = event.clientX - drag.startX;
      const directed = edge === 'right' ? delta : -delta;
      onResize(clamp(drag.startWidth + directed));
    },
    [clamp, edge, onResize]
  );

  const finishDrag = useCallback((event: React.PointerEvent<HTMLDivElement>) => {
    const drag = dragRef.current;
    if (!drag || drag.pointerId !== event.pointerId) return;
    dragRef.current = null;
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
  }, []);

  const handleKeyDown = useCallback(
    (event: React.KeyboardEvent<HTMLDivElement>) => {
      let next: number | null = null;
      const current = getStartWidth();
      if (event.key === 'ArrowLeft') {
        next = edge === 'right' ? current - step : current + step;
      } else if (event.key === 'ArrowRight') {
        next = edge === 'right' ? current + step : current - step;
      } else if (event.key === 'Home') {
        next = min;
      } else if (event.key === 'End') {
        next = max();
      }
      if (next === null) return;
      event.preventDefault();
      onResize(clamp(next));
    },
    [clamp, edge, getStartWidth, max, min, onResize, step]
  );

  return (
    <div
      role="separator"
      aria-orientation="vertical"
      aria-label={ariaLabel}
      aria-valuemin={min}
      aria-valuenow={Math.round(getStartWidth())}
      aria-valuetext={`${Math.round(getStartWidth())} pixels`}
      tabIndex={0}
      data-testid="resizable-handle"
      onPointerDown={handlePointerDown}
      onPointerMove={handlePointerMove}
      onPointerUp={finishDrag}
      onPointerCancel={finishDrag}
      onKeyDown={handleKeyDown}
      className={cn(
        'group/handle relative z-10 flex h-full w-1 shrink-0 touch-none cursor-ew-resize items-center justify-center bg-transparent outline-none select-none',
        // Wider invisible hit area so the 1px strip is actually grabbable.
        'before:absolute before:inset-y-0 before:-left-1.5 before:-right-1.5 before:content-[""]',
        // ZCode grip line: hairline round cap, visible on hover/drag/focus only.
        "after:pointer-events-none after:absolute after:inset-y-2 after:w-0.5 after:rounded-full after:bg-foreground-subtlest/60 after:opacity-0 after:transition-opacity after:content-['']",
        'hover:after:opacity-100 active:after:opacity-100 focus-visible:after:opacity-100',
        className
      )}
    />
  );
};
