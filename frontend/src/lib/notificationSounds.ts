import completionUrl from '../assets/notification-completion.mp3';
import inputUrl from '../assets/notification-input.mp3';

// Notification sounds (t3code assets, MIT). Played on turn boundaries; users
// can mute them from the Appearance panel.
export type NotificationSoundKind = 'completion' | 'input';

const SOURCES: Record<NotificationSoundKind, string> = {
  completion: completionUrl,
  input: inputUrl,
};

const SOUND_ENABLED_KEY = 'forge:notification-sound';
const VOLUME = 0.35;

let enabledCache: boolean | null = null;

export function isNotificationSoundEnabled(): boolean {
  if (enabledCache !== null) return enabledCache;
  try {
    enabledCache = localStorage.getItem(SOUND_ENABLED_KEY) !== 'off';
  } catch {
    enabledCache = true;
  }
  return enabledCache;
}

export function setNotificationSoundEnabled(enabled: boolean) {
  enabledCache = enabled;
  try {
    localStorage.setItem(SOUND_ENABLED_KEY, enabled ? 'on' : 'off');
  } catch {
    // ignore
  }
}

const audioCache = new Map<NotificationSoundKind, HTMLAudioElement>();

function getAudio(kind: NotificationSoundKind): HTMLAudioElement | null {
  let audio = audioCache.get(kind);
  if (!audio) {
    try {
      audio = new Audio(SOURCES[kind]);
      audio.preload = 'auto';
      audio.volume = VOLUME;
      audioCache.set(kind, audio);
    } catch {
      return null;
    }
  }
  return audio;
}

/** Fire-and-forget playback; safe to call from event handlers only. */
export function playNotificationSound(kind: NotificationSoundKind) {
  if (!isNotificationSoundEnabled()) return;
  const audio = getAudio(kind);
  if (!audio) return;
  try {
    audio.currentTime = 0;
    void audio.play().catch(() => {
      // Autoplay policy: a user gesture has not happened yet in this session.
    });
  } catch {
    // ignore
  }
}
