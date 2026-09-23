import React, { useState } from 'react';
import { Search, X } from 'lucide-react';
import { cn } from '../../lib/utils';

/**
 * Settings page primitives ported 1:1 from ZCode SettingsPageParts:
 * framed group cards, label/description + control rows, badges,
 * and the shared search input.
 */

export const SETTINGS_FRAME_CONTENT_CLASSNAME =
  'mx-auto w-full max-w-4xl px-4 pb-8 pt-0 lg:px-8 lg:pb-10';

export function Switch({
  checked,
  onCheckedChange,
  disabled,
  testId
}: {
  checked: boolean;
  onCheckedChange: (checked: boolean) => void;
  disabled?: boolean;
  testId?: string;
}) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      data-state={checked ? 'checked' : 'unchecked'}
      data-testid={testId}
      disabled={disabled}
      className={cn(
        'inline-flex h-5 w-9 shrink-0 items-center rounded-full border-2 border-transparent shadow-sm transition-colors disabled:cursor-not-allowed disabled:opacity-50',
        checked ? 'bg-primary' : 'bg-input'
      )}
      onClick={() => onCheckedChange(!checked)}
    >
      <span
        data-state={checked ? 'checked' : 'unchecked'}
        className={cn(
          'pointer-events-none block size-4 rounded-full bg-background shadow transition-transform',
          checked ? 'translate-x-4' : 'translate-x-0'
        )}
      />
    </button>
  );
}

export function SettingsGroupCard({
  children,
  className
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <div
      className={cn(
        'overflow-hidden rounded-xl border border-border bg-card py-0 shadow-none',
        className
      )}
    >
      {children}
    </div>
  );
}

export function SettingsRow({
  label,
  description,
  control,
  detail,
  controlLayout = 'default'
}: {
  label: React.ReactNode;
  description?: React.ReactNode;
  control: React.ReactNode;
  detail?: React.ReactNode;
  controlLayout?: 'default' | 'wide';
}) {
  return (
    <div className="border-t border-border px-4 py-3 first:border-t-0">
      <div
        className={cn(
          'grid items-center gap-4',
          controlLayout === 'wide'
            ? 'grid-cols-1 sm:grid-cols-[minmax(0,1fr)_280px]'
            : 'grid-cols-[minmax(0,1fr)_192px]'
        )}
      >
        <div className="min-w-0">
          <div className="text-ui-base font-medium text-foreground">{label}</div>
          {description ? (
            <div className="mt-1 text-ui-base leading-6 text-foreground-subtle">{description}</div>
          ) : null}
        </div>
        <div className="flex w-full flex-nowrap items-center justify-end gap-2">
          {controlLayout === 'wide' ? detail : null}
          {control}
        </div>
      </div>
      {detail && controlLayout !== 'wide' ? <div className="mt-3">{detail}</div> : null}
    </div>
  );
}

export function SettingsBadge({ children }: { children: React.ReactNode }) {
  return (
    <span className="rounded-md bg-surface px-2.5 py-1 text-ui-base font-medium text-foreground-subtle">
      {children}
    </span>
  );
}

/** Scope badge for resource rows (user / workspace / plugin). */
export function SettingsScopeBadge({ scope }: { scope: string }) {
  return (
    <span className="shrink-0 rounded-full border border-border px-2 py-0.5 text-ui-xs text-foreground-subtle">
      {scope}
    </span>
  );
}

/** Shared search input (SettingsSearchInput). */
export function SettingsSearchInput({
  value,
  onChange,
  placeholder,
  testId,
  className
}: {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  testId?: string;
  className?: string;
}) {
  const canClear = value.length > 0;
  return (
    <div className={cn('relative', className)}>
      <Search
        className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-foreground-subtle"
        aria-hidden="true"
      />
      <input
        type="search"
        data-testid={testId}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder}
        className="h-9 w-full rounded-xl border border-border bg-background pl-9 pr-9 text-ui-sm text-foreground outline-none transition-colors placeholder:text-foreground-subtlest hover:border-input-border-hover focus:border-input-border-focused [&::-webkit-search-cancel-button]:appearance-none"
      />
      {canClear ? (
        <button
          type="button"
          aria-label="Clear search"
          className="absolute right-1 top-1/2 flex size-7 -translate-y-1/2 items-center justify-center rounded-full text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground"
          onClick={() => onChange('')}
        >
          <X className="size-3.5" aria-hidden="true" />
        </button>
      ) : null}
    </div>
  );
}

/** Resource list container: surface card with hairline separators. */
export function SettingsResourceList({ children }: { children: React.ReactNode }) {
  return <div className="overflow-hidden rounded-xl bg-surface">{children}</div>;
}

export function SettingsResourceSeparator() {
  return <div className="h-px bg-border/50" role="separator" />;
}

/** Empty state for resource lists. */
export function SettingsEmptyState({ children }: { children: React.ReactNode }) {
  return (
    <div className="rounded-xl border border-dashed border-border px-4 py-8 text-center text-foreground-subtle">
      {children}
    </div>
  );
}

/** Section title heading used at the top of each settings section. */
export function SectionTitle({
  title,
  badge,
  children
}: {
  title: string;
  badge?: string;
  children?: React.ReactNode;
}) {
  return (
    <div className="flex items-center gap-3">
      <h2 className="text-2xl font-semibold tracking-tight text-foreground lg:text-3xl">{title}</h2>
      {badge ? <BetaBadge label={badge} /> : null}
      {children}
    </div>
  );
}

export function BetaBadge({ label }: { label: string }) {
  return (
    <span className="inline-flex h-6 items-center rounded-full border border-sky-500 px-2 text-ui-xs font-semibold text-sky-500">
      {label}
    </span>
  );
}

/** Generic select styled like the reference SelectTrigger. */
export function SettingsSelect({
  value,
  onChange,
  options,
  className,
  testId
}: {
  value: string;
  onChange: (value: string) => void;
  options: Array<{ value: string; label: string }>;
  className?: string;
  testId?: string;
}) {
  return (
    <select
      data-testid={testId}
      value={value}
      onChange={(event) => onChange(event.target.value)}
      className={cn(
        'h-9 rounded-xl border border-border bg-background px-3 pr-8 text-ui-sm text-foreground outline-none transition-colors hover:border-input-border-hover focus:border-input-border-focused',
        className
      )}
    >
      {options.map((option) => (
        <option key={option.value} value={option.value}>
          {option.label}
        </option>
      ))}
    </select>
  );
}

/** Standard button styles used across settings forms. */
export const settingsButtonClasses = {
  primary:
    'inline-flex h-9 items-center justify-center gap-1.5 whitespace-nowrap rounded-lg bg-brand px-4 text-ui-sm font-medium text-foreground-inverse transition-colors hover:bg-brand/80 disabled:opacity-60',
  outline:
    'inline-flex h-9 items-center justify-center gap-1.5 whitespace-nowrap rounded-lg border border-border px-4 text-ui-sm font-medium text-foreground transition-colors hover:bg-surface-hover disabled:opacity-60',
  ghost:
    'inline-flex h-9 items-center justify-center gap-1.5 whitespace-nowrap rounded-lg px-3 text-ui-sm font-medium text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground disabled:opacity-60',
  input:
    'w-full rounded-lg border border-border bg-background px-3 py-2 text-ui-sm text-foreground outline-none transition-colors placeholder:text-foreground-subtlest focus:border-input-border-focused'
};
