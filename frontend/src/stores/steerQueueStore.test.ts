import { describe, it, expect, beforeEach } from 'vitest';
import { useSteerQueueStore } from './steerQueueStore';

describe('steerQueueStore', () => {
  beforeEach(() => {
    localStorage.clear();
    useSteerQueueStore.setState({ queues: {} });
  });

  it('keeps queues completely isolated between sessions', () => {
    const store = useSteerQueueStore.getState();
    store.addToQueue('sess-A', 'instruction A1');
    store.addToQueue('sess-A', 'instruction A2');

    store.addToQueue('sess-B', 'instruction B1');

    expect(useSteerQueueStore.getState().getQueue('sess-A')).toEqual([
      'instruction A1',
      'instruction A2'
    ]);
    expect(useSteerQueueStore.getState().getQueue('sess-B')).toEqual([
      'instruction B1'
    ]);
    expect(useSteerQueueStore.getState().getQueue('sess-C')).toEqual([]);
  });

  it('removes item by index from specific session only', () => {
    const store = useSteerQueueStore.getState();
    store.addToQueue('sess-A', 'first');
    store.addToQueue('sess-A', 'second');
    store.addToQueue('sess-A', 'third');

    store.removeFromQueue('sess-A', 1); // remove 'second'

    expect(useSteerQueueStore.getState().getQueue('sess-A')).toEqual(['first', 'third']);
  });

  it('reorders items within session queue', () => {
    const store = useSteerQueueStore.getState();
    store.addToQueue('sess-A', 'item 0');
    store.addToQueue('sess-A', 'item 1');
    store.addToQueue('sess-A', 'item 2');

    store.moveInQueue('sess-A', 0, 2);

    expect(useSteerQueueStore.getState().getQueue('sess-A')).toEqual([
      'item 1',
      'item 2',
      'item 0'
    ]);
  });

  it('updates an existing item in queue', () => {
    const store = useSteerQueueStore.getState();
    store.addToQueue('sess-A', 'original text');

    store.updateQueueItem('sess-A', 0, 'edited text');

    expect(useSteerQueueStore.getState().getQueue('sess-A')).toEqual(['edited text']);
  });

  it('clears session queue without touching other sessions', () => {
    const store = useSteerQueueStore.getState();
    store.addToQueue('sess-A', 'A');
    store.addToQueue('sess-B', 'B');

    store.clearQueue('sess-A');

    expect(useSteerQueueStore.getState().getQueue('sess-A')).toEqual([]);
    expect(useSteerQueueStore.getState().getQueue('sess-B')).toEqual(['B']);
  });
});
