import React, {
  createContext,
  useContext,
  useLayoutEffect,
  useRef,
  useState,
  type ComponentProps,
  type ReactNode,
} from 'react';
import { ChevronRight, Loader2, X, Clock } from 'lucide-react';
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
  // Trailing indicator: quiet by default — only running spins and failures get a dim X.
  let glyph: ReactNode = null;
  if (status === 'running') {
    glyph = <Loader2 className="size-3 animate-spin text-icon-muted shrink-0" />;
  } else if (status === 'failed') {
    glyph = <X className="size-3 shrink-0 text-tool-error-icon/60" />;
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
        'flex min-h-6 min-w-0 gap-1.5 text-ui-sm leading-relaxed select-none [&_*]:select-none',
        wrapLabel ? 'items-start' : 'items-center'
      )}
    >
      <div className="flex min-w-0 flex-1 items-center gap-1.5">
        {icon ? (
          <span className="relative w-6 shrink-0 leading-0">
            <span className="inline-block size-4 shrink-0 overflow-hidden align-[calc(0.5cap_-_0.75rem)] flex items-center justify-center text-icon-muted">
              {icon}
            </span>
          </span>
        ) : null}
        <div
          className={cn(
            'min-w-0 flex-1 text-secondary-label',
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
  'cursor-pointer hover:bg-accent/70 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring/70';

export function WorkLogButton({
  icon,
  label,
  trailing,
  wrapLabel = false,
  ...buttonProps
}: RowContent & Omit<ComponentProps<'button'>, 'className' | 'style' | 'children'>) {
  const grouped = useContext(GroupedRows);
  const className = cn(
    'group/timeline-row relative w-full min-w-0 rounded-md px-0.5 text-left transition-colors',
    grouped ? 'py-0' : 'py-0.5',
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
        'group/timeline-row relative w-full min-w-0 rounded-md px-0.5 text-left transition-colors',
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
          'flex min-h-6 w-full min-w-0 items-center justify-between gap-1.5 rounded-md py-0.5 text-ui-sm leading-relaxed select-none transition-colors',
          hasChildren && interactionClassName,
          grouped ? 'py-0' : 'py-0.5'
        )}
      >
        <div className="flex min-w-0 flex-1 items-center gap-1.5">
          <span
            className={cn(
              'flex size-4 shrink-0 items-center justify-center',
              !hasChildren && 'invisible'
            )}
          >
            <ChevronRight
              className={cn(
                'size-3 shrink-0 text-icon-muted opacity-70 transition-transform duration-200',
                expanded && 'rotate-90'
              )}
            />
          </span>
          <div className="flex min-w-0 flex-1 items-center gap-1.5">
            {icon ? (
              <span className="relative w-6 shrink-0 leading-0">
                <span className="inline-flex size-4 shrink-0 items-center justify-center overflow-hidden align-[calc(0.5cap_-_0.75rem)] text-icon-muted">
                  {icon}
                </span>
              </span>
            ) : null}
            <div
              className={cn(
                'min-w-0 flex-1 text-secondary-label',
                !wrapLabel && 'truncate [&_*]:whitespace-nowrap'
              )}
            >
              {label}
            </div>
          </div>
        </div>
        {trailing}
      </div>

      {expanded && children && (
        <div className="mt-0.5 mb-1.5 pb-2">{children}</div>
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
        'cursor-auto overflow-auto font-mono text-[length:var(--font-size-code,0.6875rem)] leading-relaxed',
        kind === 'text' &&
          'ms-7 max-h-96 px-0.5 py-1 text-secondary-label select-text whitespace-pre-wrap break-words scrollbar-thin',
        kind === 'panel' && 'ms-7 mt-0.5 mb-1.5 pb-2',
        kind === 'media' && 'ms-7 mt-1 flex items-center justify-center',
        className
      )}
      onClick={event => event.stopPropagation()}
      onPointerDown={event => event.stopPropagation()}
    >
      {children}
    </div>
  );
}
