import { useState, useEffect, useRef } from 'react';
import { playNotificationSound } from '../../../lib/notificationSounds';

export interface UseAgentExecutionResult {
  liveElapsed: number;
}

/**
 * Tracks agent session execution duration and plays completion notifications.
 */
export function useAgentExecution(
  isSessionWorking: boolean,
  lastAgentMsgId?: string
): UseAgentExecutionResult {
  const [liveElapsed, setLiveElapsed] = useState<number>(0);
  const prevWorkingRef = useRef<boolean>(false);

  useEffect(() => {
    if (!isSessionWorking) {
      setLiveElapsed(0);
      return;
    }
    const start = Date.now();
    setLiveElapsed(0);
    const interval = setInterval(() => {
      setLiveElapsed(Math.max(1, Math.floor((Date.now() - start) / 1000)));
    }, 1000);
    return () => clearInterval(interval);
  }, [isSessionWorking, lastAgentMsgId]);

  useEffect(() => {
    if (prevWorkingRef.current && !isSessionWorking) {
      playNotificationSound('completion');
    }
    prevWorkingRef.current = isSessionWorking;
  }, [isSessionWorking]);

  return { liveElapsed };
}
