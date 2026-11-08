import { describe, it, expect, beforeEach } from 'vitest';
import { useToastStore } from '../toastStore';

describe('toast store', () => {
  beforeEach(() => {
    useToastStore.setState({ toasts: [] });
  });

  it('queues a toast with a stable id and tone', () => {
    const id = useToastStore
      .getState()
      .push({ title: 'Transaction saved', tone: 'success' });

    const [toast] = useToastStore.getState().toasts;
    expect(useToastStore.getState().toasts).toHaveLength(1);
    expect(toast).toEqual({ id, title: 'Transaction saved', tone: 'success' });
  });

  it('keeps optional descriptions', () => {
    useToastStore
      .getState()
      .push({ title: 'Could not save', description: 'Try again', tone: 'error' });

    expect(useToastStore.getState().toasts[0].description).toBe('Try again');
  });

  it('dismisses only the requested toast', () => {
    const first = useToastStore.getState().push({ title: 'One', tone: 'info' });
    const second = useToastStore.getState().push({ title: 'Two', tone: 'info' });

    useToastStore.getState().dismiss(first);

    expect(useToastStore.getState().toasts).toEqual([
      { id: second, title: 'Two', tone: 'info' },
    ]);
  });
});
