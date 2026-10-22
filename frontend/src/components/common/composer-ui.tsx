import React from 'react';
import { Check, ChevronDown, Search } from 'lucide-react';
import { cn } from '../../lib/utils';

/**
 * Composer control primitives for ForgeADE.
 * These are the chip / menu recipes shared by the composer footer, the model
 * / agent / effort pickers, the workspace selector and the branch selector.
 */

// ---------------------------------------------------------------------------
// ComposerControl — the chip used for every composer footer button.
// Sizes: "sm" (footer pickers) and "xs" (context chips: workspace, branch).
// ---------------------------------------------------------------------------

const CHIP_BASE =
  'relative inline-flex shrink-0 cursor-pointer items-center justify-center whitespace-nowrap rounded-md border border-transparent font-medium outline-none transition-colors hover:bg-accent disabled:pointer-events-none disabled:opacity-60 active:scale-[0.97] [&_svg]:pointer-events-none [&_svg]:shrink-0';

const CHIP_SIZES = {
  sm: 'h-7 gap-1.5 px-2.5 text-ui-base text-secondary-label hover:text-foreground [&_svg:not([class*="text-"]):not([class*="opacity-"])]:text-muted-foreground [&_svg:not([class*="size-"])]:size-4',
  xs: 'h-7 gap-1 px-1.75 text-ui-xs font-normal text-muted-foreground/70 hover:text-foreground/80 sm:h-6 [&_svg:not([class*="size-"])]:size-3.5',
} as const;

export interface ComposerControlProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  size?: keyof typeof CHIP_SIZES;
  /** Open/pressed state. */
  pressed?: boolean;
}

export const ComposerControl = React.forwardRef<HTMLButtonElement, ComposerControlProps>(
  ({ size = 'sm', pressed, className, type, ...props }, ref) => (
    <button
      ref={ref}
      type={type ?? 'button'}
      className={cn(
        CHIP_BASE,
        CHIP_SIZES[size],
        pressed && 'bg-accent text-foreground',
        className,
      )}
      {...props}
    />
  ),
);
ComposerControl.displayName = 'ComposerControl';

/** The little dropdown chevron every picker chip carries. */
export const ComposerChevron: React.FC<{ size?: 'sm' | 'xs'; className?: string }> = ({
  size = 'sm',
  className,
}) => (
  <ChevronDown
    aria-hidden="true"
    strokeWidth={2.25}
    className={cn(
      size === 'sm' ? 'size-3.5 text-icon-muted' : 'size-3 opacity-50',
      className,
    )}
  />
);

// ---------------------------------------------------------------------------
// Menus — glass panel (dropdown-glass) + combobox row recipe.
// ---------------------------------------------------------------------------

export const composerMenuClass =
  'dropdown-glass z-50 animate-in rounded-xl border border-border bg-popover/98 backdrop-blur-xl p-1 text-ui-sm text-fg-primary shadow-2xl fade-in zoom-in-95 duration-100';

export const ComposerMenu: React.FC<
  React.HTMLAttributes<HTMLDivElement> & { role?: string; label?: string }
> = ({ role = 'menu', label, className, children, ...props }) => (
  <div role={role} aria-label={label} className={cn(composerMenuClass, className)} {...props}>
    {children}
  </div>
);

export const ComposerMenuItem: React.FC<
  React.ButtonHTMLAttributes<HTMLButtonElement> & { selected?: boolean }
> = ({ selected, className, children, type, ...props }) => (
  <button
    type={type ?? 'button'}
    aria-checked={props.role === 'menuitemradio' ? selected : undefined}
    className={cn(
      'flex min-h-7 w-full cursor-pointer items-center gap-2 rounded-sm px-2 py-1 text-left text-ui-base text-fg-primary outline-none transition-colors hover:bg-accent',
      selected && 'bg-foreground/[0.08]',
      className,
    )}
    {...props}
  >
    {children}
  </button>
);

export const ComposerMenuLabel: React.FC<React.HTMLAttributes<HTMLDivElement>> = ({
  className,
  ...props
}) => (
  <div
    className={cn(
      'px-2 py-1 text-2xs font-semibold uppercase tracking-wider text-fg-tertiary',
      className,
    )}
    {...props}
  />
);

/** Underline search row (no box — the bottom border carries the field). */
export const ComposerMenuSearch: React.FC<{
  value: string;
  onChange: (next: string) => void;
  placeholder?: string;
  autoFocus?: boolean;
  inputRef?: React.RefObject<HTMLInputElement | null>;
  ariaLabel?: string;
}> = ({ value, onChange, placeholder, autoFocus, inputRef, ariaLabel }) => (
  <div className="border-b border-border-subtle px-3 pt-2.5 pb-1.5">
    <div className="relative -translate-y-px transition-colors focus-within:border-ring">
      <Search className="absolute top-1/2 left-0 size-3.5 -translate-y-1/2 text-muted-foreground/55" />
      <input
        ref={inputRef}
        type="text"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        aria-label={ariaLabel ?? placeholder}
        autoFocus={autoFocus}
        className="h-6.5 w-full bg-transparent pl-5 font-sans text-ui-sm text-fg-primary outline-none placeholder:text-fg-tertiary"
      />
    </div>
  </div>
);

export const ComposerMenuCheck: React.FC<{ className?: string }> = ({ className }) => (
  <Check aria-hidden="true" className={cn('size-3.5 shrink-0 text-fg-primary', className)} />
);

/** Keyboard hint chip. */
export const ComposerKbd: React.FC<React.HTMLAttributes<HTMLSpanElement>> = ({
  className,
  ...props
}) => (
  <span
    className={cn(
      'inline-flex h-5 min-w-5 items-center justify-center rounded bg-muted px-1 font-sans text-2xs font-medium text-muted-foreground',
      className,
    )}
    {...props}
  />
);
