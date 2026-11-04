import { useSteerQueueStore } from '../../stores/steerQueueStore';
import { showToast } from '../../lib/toast';

type PromptSender = (prompt: string) => void;
type SessionFinder = (sessionId: string) => { id: string; status: string } | undefined;

let promptSender: PromptSender | null = null;
let sessionFinder: SessionFinder | null = null;

export function registerSteerQueueHandlers(handlers: {
  sendPrompt: PromptSender;
  findSession: SessionFinder;
}) {
  promptSender = handlers.sendPrompt;
  sessionFinder = handlers.findSession;
}

const activeQueueProcessing = new Set<string>();

/**
 * processNextSteerQueueItem
 *
 * Checks if there is a queued steer instruction for the given session.
 * If the session is NOT running and has queued items:
 * - Pops the head of the queue
 * - Sends the prompt into the session
 *
 * Returns true if an item was popped and dispatched, false otherwise.
 */
export function processNextSteerQueueItem(sessionId: string): boolean {
  if (!sessionId || activeQueueProcessing.has(sessionId)) return false;

  const queue = useSteerQueueStore.getState().getQueue(sessionId);
  if (!queue || queue.length === 0) return false;

  const session = sessionFinder ? sessionFinder(sessionId) : undefined;
  // If session is still running, wait until agent stops!
  if (session && session.status === 'running') {
    return false;
  }

  activeQueueProcessing.add(sessionId);
  try {
    const nextInstruction = useSteerQueueStore.getState().popQueue(sessionId);
    if (!nextInstruction) {
      activeQueueProcessing.delete(sessionId);
      return false;
    }

    setTimeout(() => {
      try {
        const currSession = sessionFinder ? sessionFinder(sessionId) : undefined;
        if (!currSession || currSession.status !== 'running') {
          showToast('Steering next queued instruction...', 'info');
          if (promptSender) {
            promptSender(nextInstruction);
          }
        }
      } finally {
        activeQueueProcessing.delete(sessionId);
      }
    }, 60);

    return true;
  } catch {
    activeQueueProcessing.delete(sessionId);
    return false;
  }
}
