import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import Chat from '../Chat';
import { useChatStore } from '../../stores/chatStore';
import { chatApi } from '../../lib/chatApi';

vi.mock('../../lib/chatApi', () => ({
  chatApi: {
    send: vi.fn(),
    confirm: vi.fn(),
    cancel: vi.fn(),
  },
}));

describe('Chat', () => {
  beforeEach(() => {
    vi.mocked(chatApi.send).mockReset();
    vi.mocked(chatApi.confirm).mockReset();
    vi.mocked(chatApi.cancel).mockReset();
    useChatStore.setState({
      entries: [],
      isLoading: false,
      isConfirming: false,
      error: null,
    });
  });

  it('shows an empty state with suggestions', () => {
    render(<Chat />);

    expect(
      screen.getByText(/ask me anything about your finances/i)
    ).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: /what are my account balances/i })
    ).toBeInTheDocument();
  });

  it('sends a message and renders the reply with tool chips', async () => {
    const user = userEvent.setup();
    vi.mocked(chatApi.send).mockResolvedValue({
      reply: 'You have one account: Cash.',
      tool_calls: [{ name: 'list_accounts', summary: 'Listed 1 accounts' }],
    });

    render(<Chat />);

    await user.type(screen.getByLabelText('Message'), 'Which accounts do I have?');
    await user.click(screen.getByRole('button', { name: /send/i }));

    expect(await screen.findByText('You have one account: Cash.')).toBeInTheDocument();
    expect(screen.getByText('list accounts')).toBeInTheDocument();
    expect(chatApi.send).toHaveBeenCalledWith([
      { role: 'user', content: 'Which accounts do I have?' },
    ]);
  });

  it('requires confirmation before running a destructive action', async () => {
    const user = userEvent.setup();
    vi.mocked(chatApi.send).mockResolvedValue({
      reply: 'Delete "Coffee" 12.50 on 2026-10-01?',
      tool_calls: [{ name: 'delete_transaction', summary: 'Delete Coffee' }],
      pending_action: { id: 'pending-1', summary: 'Delete "Coffee" 12.50 on 2026-10-01?' },
    });
    vi.mocked(chatApi.confirm).mockResolvedValue({
      reply: 'Deleted.',
      tool_calls: [],
    });

    render(<Chat />);

    await user.type(screen.getByLabelText('Message'), 'Delete my coffee expense');
    await user.click(screen.getByRole('button', { name: /send/i }));

    const confirmButton = await screen.findByRole('button', { name: /^confirm$/i });
    expect(chatApi.confirm).not.toHaveBeenCalled();

    await user.click(confirmButton);

    expect(chatApi.confirm).toHaveBeenCalledWith('pending-1');
    expect(await screen.findByText('Deleted.')).toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: /^confirm$/i })
    ).not.toBeInTheDocument();
  });

  it('surfaces assistant errors', async () => {
    const user = userEvent.setup();
    vi.mocked(chatApi.send).mockRejectedValue(new Error('chat is not configured'));

    render(<Chat />);

    await user.type(screen.getByLabelText('Message'), 'hello');
    await user.click(screen.getByRole('button', { name: /send/i }));

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'chat is not configured'
    );
  });
});
