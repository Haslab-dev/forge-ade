import React, { useEffect, useState } from 'react';
import { Blocks } from 'lucide-react';
import { cn } from '../../../lib/utils';
import { isTrustedImageUrl } from './pluginStoreListing';

/**
 * Store entry avatar: https listing.icon preferred, falls back to a neutral
 * Blocks icon inside a bg-surface rounded square (ZCode PluginStoreAvatar).
 */
export function PluginStoreAvatar({
  src,
  pluginId,
  className,
  iconClassName
}: {
  src?: string;
  pluginId?: string;
  className?: string;
  iconClassName?: string;
}) {
  const trusted = isTrustedImageUrl(src);
  const [failed, setFailed] = useState(false);
  useEffect(() => setFailed(false), [src]);
  const showImage = trusted && !failed;

  return (
    <span
      data-testid={pluginId ? `plugin-store-avatar:${pluginId}` : undefined}
      className={cn(
        'relative flex shrink-0 items-center justify-center overflow-hidden rounded-lg bg-surface',
        className ?? 'size-10'
      )}
    >
      {showImage ? (
        <img
          src={src}
          alt=""
          aria-hidden="true"
          draggable={false}
          className={cn('size-full object-contain p-1', iconClassName)}
          onError={() => setFailed(true)}
        />
      ) : (
        <Blocks aria-hidden="true" className={cn('size-5 text-foreground-subtle', iconClassName)} />
      )}
    </span>
  );
}
