import React, {
  createContext,
  useContext,
  useLayoutEffect,
  useRef,
  useState,
  type ComponentProps,
  type ReactNode,
} from 'react';
import { ChevronRight, Loader2, CheckCircle2, AlertCircle, Clock } from 'lucide-react';
import { cn } from '../../lib/utils';

const GroupedRows = createContext(false);

/** Groups may span virtualized timeline items. Each part owns its trailing space. */
export function WorkLogBlock({
  layout = 'standalone',
  continues = false,
  children,
  className,
}: {
  layout?: 'standalone' | 'group-header' | 'group-content';
  continues?: boolean;
  children: ReactNode;
  className?: string;
}) {
  return (
    <div
      className={cn(
        continues || layout === 'group-header'
          ? 'pb-0'
          : layout === 'group-content'
            ? 'pb-1'
            : 'pb-2',
        className
      )}
    >
      {children}
    </div>
  );
}

export function WorkLogList({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <GroupedRows.Provider value={true}>
      <div className={cn('flex min-w-0 flex-col gap-0.5', className)}>{children}</div>
    </GroupedRows.Provider>
  );
}

export type WorkLogStatus = 'pending' | 'running' | 'completed' | 'failed' | 'warning';

export interface WorkLogStatusGlyphProps {
  status?: WorkLogStatus;
  durationMs?: number;
}

export function WorkLogStatusGlyph({ status, durationMs }: WorkLogStatusGlyphProps) {
  let glyph: ReactNode = null;
  if (status === 'running') {
    glyph = <Loader2 className="size-3.5 animate-spin text-accent-primary shrink-0" />;
  } else if (status === 'completed') {
    glyph = <CheckCircle2 className="size-3.5 text-status-success shrink-0" />;
  } else if (status === 'failed') {
    glyph = <AlertCircle className="size-3.5 text-status-danger shrink-0" />;
  }

  const durationText = durationMs && durationMs > 0
    ? durationMs < 1000
      ? `${durationMs}ms`
      : `${(durationMs / 1000).toFixed(1)}s`
    : null;

  return (
    <div className="flex items-center gap-1.5 text-2xs text-secondary-label select-none shrink-0">
      {durationText && (
        <span className="font-mono text-3xs text-fg-muted flex items-center gap-0.5">
          <Clock className="size-2.5 opacity-70" />
          {durationText}
        </span>
      )}
      {glyph}
    </div>
  );
}

type RowContent = {
  icon?: ReactNode;
  label: ReactNode;
  trailing?: ReactNode;
  wrapLabel?: boolean;
};

function WorkLogLine({ icon, label, trailing, wrapLabel }: RowContent) {
  return (
    <div
      className={cn(
        'flex min-h-6 min-w-0 gap-1.5 text-ui-xs leading-relaxed select-none',
        wrapLabel ? 'items-start' : 'items-center'
      )}
    >
      <div className="flex min-w-0 flex-1 items-center gap-2">
        {icon ? (
          <span className="relative size-4 shrink-0 flex items-center justify-center text-fg-secondary">
            {icon}
          </span>
        ) : null}
        <div
          className={cn(
            'min-w-0 flex-1 text-fg-secondary font-mono text-ui-xs',
            !wrapLabel && 'truncate [&_*]:whitespace-nowrap'
          )}
        >
          {label}
        </div>
      </div>
      {trailing}
    </div>
  );
}

const interactionClassName =
  'cursor-pointer focus-visible:outline-none focus-visible:ring-1.5 focus-visible:ring-accent-primary/70 hover:bg-surface-hover';

export function WorkLogButton({
  icon,
  label,
  trailing,
  wrapLabel = false,
  ...buttonProps
}: RowContent & Omit<ComponentProps<'button'>, 'className' | 'style' | 'children'>) {
  const grouped = useContext(GroupedRows);
  const className = cn(
    'group/timeline-row relative w-full min-w-0 rounded-md px-1.5 text-left transition-colors',
    grouped ? 'py-0.5' : 'py-1',
    interactionClassName
  );
  return (
    <button {...buttonProps} type="button" className={className}>
      <WorkLogLine icon={icon} label={label} trailing={trailing} wrapLabel={wrapLabel} />
    </button>
  );
}

/** Expandable disclosure row for tool calls with live outputs, diffs, or details */
export function WorkLogRow({
  icon,
  label,
  trailing,
  wrapLabel = false,
  children,
  defaultExpanded = false,
  className,
}: RowContent & {
  children?: ReactNode;
  defaultExpanded?: boolean;
  className?: string;
}) {
  const [expanded, setExpanded] = useState(defaultExpanded);
  const hasChildren = Boolean(children);
  const grouped = useContext(GroupedRows);

  return (
    <div
      className={cn(
        'group/timeline-row relative w-full min-w-0 rounded-lg border border-transparent transition-all',
        expanded && 'bg-surface/50 border-border/40 my-1',
        className
      )}
    >
      <div
        role={hasChildren ? 'button' : undefined}
        tabIndex={hasChildren ? 0 : undefined}
        onClick={() => hasChildren && setExpanded(prev => !prev)}
        onKeyDown={e => {
          if (hasChildren && (e.key === 'Enter' || e.key === ' ')) {
            e.preventDefault();
            setExpanded(prev => !prev);
          }
        }}
        className={cn(
          'w-full flex items-center justify-between rounded-md px-2 py-1 select-none transition-colors',
          hasChildren && 'cursor-pointer hover:bg-surface-hover',
          grouped ? 'py-0.5' : 'py-1'
        )}
      >
        <div className="flex min-w-0 flex-1 items-center gap-1.5">
          {hasChildren && (
            <ChevronRight
              className={cn(
                'size-3.5 text-fg-muted shrink-0 transition-transform duration-150',
                expanded && 'rotate-90 text-fg-primary'
              )}
            />
          )}
          <div className="flex min-w-0 flex-1 items-center gap-2">
            {icon ? (
              <span className="size-4 shrink-0 flex items-center justify-center text-fg-secondary">
                {icon}
              </span>
            ) : null}
            <div
              className={cn(
                'min-w-0 flex-1 text-fg-primary font-mono text-ui-xs truncate',
                !wrapLabel && 'truncate'
              )}
            >
              {label}
            </div>
          </div>
        </div>
        {trailing}
      </div>

      {expanded && children && (
        <div className="px-2 pb-2 pt-1 border-t border-border/30 animate-in fade-in-50 duration-100">
          {children}
        </div>
      )}
    </div>
  );
}

const LIVE_EDGE_PX = 8;

export function WorkLogDetails({
  children,
  kind = 'text',
  followLive = false,
  className,
}: {
  children: ReactNode;
  kind?: 'text' | 'panel' | 'media';
  followLive?: boolean;
  className?: string;
}) {
  const scrollerRef = useRef<HTMLDivElement>(null);
  const atLiveEdgeRef = useRef(true);

  useLayoutEffect(() => {
    const scroller = scrollerRef.current;
    if (followLive && atLiveEdgeRef.current && scroller) {
      scroller.scrollTop = scroller.scrollHeight;
    }
  });

  return (
    <div
      ref={scrollerRef}
      onScroll={event => {
        const { scrollHeight, scrollTop, clientHeight } = event.currentTarget;
        atLiveEdgeRef.current = scrollHeight - scrollTop - clientHeight <= LIVE_EDGE_PX;
      }}
      className={cn(
        'cursor-auto overflow-auto font-mono text-ui-xs',
        kind === 'text' &&
          'max-h-72 rounded-md bg-code-background/70 border border-border/40 p-2 text-fg-primary select-text whitespace-pre-wrap break-words scrollbar-thin',
        kind === 'panel' && 'mt-1',
        kind === 'media' && 'mt-1 flex items-center justify-center',
        className
      )}
      onClick={event => event.stopPropagation()}
      onPointerDown={event => event.stopPropagation()}
    >
      {children}
    </div>
  );
}
