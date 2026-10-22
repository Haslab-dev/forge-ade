import { useState, useEffect, useRef, useCallback, type RefObject } from 'react';
import { playNotificationSound } from '../../../lib/notificationSounds';

export interface UseAgentAutoScrollOptions {
  containerRef: RefObject<HTMLDivElement | null>;
  messages?: any[];
  status?: string;
  isVisible?: boolean;
}

export interface UseAgentAutoScrollResult {
  isScrolledUp: boolean;
  scrollToBottom: () => void;
  handleChatScroll: () => void;
  userScrolledUpRef: { current: boolean };
}

/**
 * Manages conversation auto-scroll, detects user manual scroll intent,
 * and skips layout operations when the view is not visible.
 */
export function useAgentAutoScroll({
  containerRef,
  messages,
  status,
  isVisible = true,
}: UseAgentAutoScrollOptions): UseAgentAutoScrollResult {
  const [isScrolledUp, setIsScrolledUp] = useState<boolean>(false);
  const userScrolledUpRef = useRef(false);
  const wasAtBottomRef = useRef(true);
  const lastMsgCountRef = useRef(0);

  const scrollToBottom = useCallback(() => {
    if (containerRef.current) {
      containerRef.current.scrollTo({
        top: containerRef.current.scrollHeight,
        behavior: 'smooth',
      });
      userScrolledUpRef.current = false;
      wasAtBottomRef.current = true;
      setIsScrolledUp(false);
    }
  }, [containerRef]);

  const handleChatScroll = useCallback(() => {
    if (!containerRef.current) return;
    const { scrollTop, scrollHeight, clientHeight } = containerRef.current;
    if (clientHeight === 0 || scrollHeight === 0) return;

    const distanceFromBottom = scrollHeight - (scrollTop + clientHeight);
    if (distanceFromBottom <= 50) {
      userScrolledUpRef.current = false;
      wasAtBottomRef.current = true;
      setIsScrolledUp(false);
    } else if (distanceFromBottom > 100) {
      userScrolledUpRef.current = true;
      wasAtBottomRef.current = false;
      setIsScrolledUp(true);
    }
  }, [containerRef]);

  // Keep auto-scroll at bottom during streaming unless user manually scrolled up
  useEffect(() => {
    if (!isVisible || !containerRef.current) return;
    if (userScrolledUpRef.current) return;

    // Use direct scroll assignment to eliminate smooth-scroll jank during chunk streaming
    containerRef.current.scrollTop = containerRef.current.scrollHeight;
  }, [messages, status, isVisible, containerRef]);

  // Snap to bottom when a new user message or turn begins
  useEffect(() => {
    const count = messages?.length || 0;
    if (count > lastMsgCountRef.current) {
      const newMsg = messages?.[count - 1];
      if (newMsg?.role === 'user') {
        playNotificationSound('input');
      }
      userScrolledUpRef.current = false;
      wasAtBottomRef.current = true;
      setIsScrolledUp(false);
      if (isVisible && containerRef.current) {
        containerRef.current.scrollTop = containerRef.current.scrollHeight;
      }
    }
    lastMsgCountRef.current = count;
  }, [messages?.length, messages, isVisible, containerRef]);

  return {
    isScrolledUp,
    scrollToBottom,
    handleChatScroll,
    userScrolledUpRef,
  };
}
