import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { act, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Toaster } from '../Toaster';
import { useToastStore } from '../../../stores/toastStore';

describe('Toaster', () => {
  beforeEach(() => {
    useToastStore.setState({ toasts: [] });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('renders a polite live region and dismisses manually', async () => {
    const user = userEvent.setup();
    render(<Toaster />);

    act(() => {
      useToastStore.getState().push({ title: 'Transaction saved', tone: 'success' });
    });

    const region = screen.getByRole('status');
    expect(region).toHaveAttribute('aria-live', 'polite');
    expect(region).toHaveTextContent('Transaction saved');

    await user.click(
      screen.getByRole('button', { name: 'Dismiss Transaction saved' })
    );

    expect(screen.queryByText('Transaction saved')).not.toBeInTheDocument();
  });

  it('announces error toasts as alerts', () => {
    render(<Toaster />);

    act(() => {
      useToastStore
        .getState()
        .push({ title: 'Unable to save', description: 'Try again', tone: 'error' });
    });

    expect(screen.getByRole('alert')).toHaveTextContent('Unable to save');
  });

  it('auto-dismisses success toasts after 4 seconds', () => {
    vi.useFakeTimers();
    render(<Toaster />);

    act(() => {
      useToastStore.getState().push({ title: 'Saved', tone: 'info' });
    });

    act(() => {
      vi.advanceTimersByTime(3_999);
    });
    expect(screen.getByText('Saved')).toBeInTheDocument();

    act(() => {
      vi.advanceTimersByTime(1);
    });
    expect(screen.queryByText('Saved')).not.toBeInTheDocument();
  });

  it('keeps error toasts for 6 seconds', () => {
    vi.useFakeTimers();
    render(<Toaster />);

    act(() => {
      useToastStore.getState().push({ title: 'Failed', tone: 'error' });
    });

    act(() => {
      vi.advanceTimersByTime(4_000);
    });
    expect(screen.getByText('Failed')).toBeInTheDocument();

    act(() => {
      vi.advanceTimersByTime(2_000);
    });
    expect(screen.queryByText('Failed')).not.toBeInTheDocument();
  });
});
