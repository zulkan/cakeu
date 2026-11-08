import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import Chat from '../Chat';
import { useChatStore } from '../../stores/chatStore';
import { chatApi } from '../../lib/chatApi';
import type { ChatFile } from '../../types/chat';

vi.mock('../../lib/chatApi', () => ({
  chatApi: {
    send: vi.fn(),
    uploadFile: vi.fn(),
    confirm: vi.fn(),
    cancel: vi.fn(),
  },
}));

const uploadedFile = {
  id: 'file-1',
  name: 'budget.xlsx',
  sheet: 'Sheet1',
  sheets: ['Sheet1'],
  sheet_rows: { Sheet1: 12 },
  rows: 12,
  columns: ['date', 'amount'],
  preview: [['2026-01-01', '12.50']],
};

const multiSheetFile = {
  ...uploadedFile,
  sheets: ['Sheet1', 'February', 'March'],
  sheet_rows: { Sheet1: 12, February: 5, March: 8 },
};

describe('Chat', () => {
  beforeEach(() => {
    vi.mocked(chatApi.send).mockReset();
    vi.mocked(chatApi.uploadFile).mockReset();
    vi.mocked(chatApi.confirm).mockReset();
    vi.mocked(chatApi.cancel).mockReset();
    useChatStore.setState({
      entries: [],
      isLoading: false,
      isConfirming: false,
      error: null,
      attachedFile: null,
      selectedSheet: null,
      isUploading: false,
      uploadError: null,
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

  it('renders markdown formatting in assistant replies', async () => {
    const user = userEvent.setup();
    vi.mocked(chatApi.send).mockResolvedValue({
      reply: '## Spending\n\n- **Food**: 120.00\n- Transport: 40.00',
      tool_calls: [],
    });

    render(<Chat />);

    await user.type(screen.getByLabelText('Message'), 'How much did I spend?');
    await user.click(screen.getByRole('button', { name: /send/i }));

    expect(
      await screen.findByRole('heading', { level: 2, name: 'Spending' })
    ).toBeInTheDocument();
    expect(screen.getByText('Food').tagName).toBe('STRONG');
    expect(screen.getAllByRole('listitem')).toHaveLength(2);
    expect(screen.queryByText(/\*\*Food\*\*/)).not.toBeInTheDocument();
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

  it('uploads a spreadsheet and shows a chip', async () => {
    const user = userEvent.setup();
    vi.mocked(chatApi.uploadFile).mockResolvedValue(uploadedFile);

    render(<Chat />);

    const file = new File(['date,amount'], 'budget.xlsx', {
      type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
    });
    await user.upload(screen.getByLabelText('Attach spreadsheet'), file);

    expect(chatApi.uploadFile).toHaveBeenCalledWith(file);
    expect(await screen.findByText('budget.xlsx')).toBeInTheDocument();
    expect(screen.getByText(/sheet sheet1/i)).toBeInTheDocument();
    expect(screen.getByText(/12 rows/i)).toBeInTheDocument();
  });

  it('sends the attached file id with each message until removed', async () => {
    const user = userEvent.setup();
    vi.mocked(chatApi.uploadFile).mockResolvedValue(uploadedFile);
    vi.mocked(chatApi.send)
      .mockResolvedValueOnce({
        reply: 'Imported 12 rows.',
        tool_calls: [],
      })
      .mockResolvedValueOnce({
        reply: 'The file has 12 rows.',
        tool_calls: [],
      });

    render(<Chat />);

    await user.upload(
      screen.getByLabelText('Attach spreadsheet'),
      new File(['date,amount'], 'budget.xlsx')
    );
    await screen.findByText('budget.xlsx');

    await user.type(screen.getByLabelText('Message'), 'Import this file');
    await user.click(screen.getByRole('button', { name: /send/i }));

    expect(await screen.findByText('Imported 12 rows.')).toBeInTheDocument();
    expect(chatApi.send).toHaveBeenLastCalledWith(
      [{ role: 'user', content: 'Import this file' }],
      'file-1'
    );

    await user.type(screen.getByLabelText('Message'), 'How many rows?');
    await user.click(screen.getByRole('button', { name: /send/i }));

    expect(await screen.findByText('The file has 12 rows.')).toBeInTheDocument();
    expect(chatApi.send).toHaveBeenLastCalledWith(
      [
        { role: 'user', content: 'Import this file' },
        { role: 'assistant', content: 'Imported 12 rows.' },
        { role: 'user', content: 'How many rows?' },
      ],
      'file-1'
    );
  });

  it('removes the attachment and stops sending its file id', async () => {
    const user = userEvent.setup();
    vi.mocked(chatApi.uploadFile).mockResolvedValue(uploadedFile);
    vi.mocked(chatApi.send).mockResolvedValue({ reply: 'Sure.', tool_calls: [] });

    render(<Chat />);

    await user.upload(
      screen.getByLabelText('Attach spreadsheet'),
      new File(['date,amount'], 'budget.xlsx')
    );
    await screen.findByText('budget.xlsx');

    await user.click(
      screen.getByRole('button', { name: /remove budget\.xlsx/i })
    );
    expect(screen.queryByText('budget.xlsx')).not.toBeInTheDocument();

    await user.type(screen.getByLabelText('Message'), 'Hello');
    await user.click(screen.getByRole('button', { name: /send/i }));

    await screen.findByText('Sure.');
    expect(chatApi.send).toHaveBeenLastCalledWith([
      { role: 'user', content: 'Hello' },
    ]);
  });

  it('selects a sheet for a multi-sheet upload and sends it without re-uploading', async () => {
    const user = userEvent.setup();
    vi.mocked(chatApi.uploadFile).mockResolvedValue(multiSheetFile);
    vi.mocked(chatApi.send).mockResolvedValue({
      reply: 'Ready.',
      tool_calls: [],
    });

    render(<Chat />);

    await user.upload(
      screen.getByLabelText('Attach spreadsheet'),
      new File(['date,amount'], 'budget.xlsx')
    );
    await screen.findByText('budget.xlsx');

    const sheetSelect = screen.getByLabelText('Sheet');
    expect(sheetSelect).toHaveValue('Sheet1');
    expect(
      screen.getByRole('option', { name: 'February' })
    ).toBeInTheDocument();

    await user.selectOptions(sheetSelect, 'February');
    expect(chatApi.uploadFile).toHaveBeenCalledTimes(1);

    await user.type(screen.getByLabelText('Message'), 'Import February');
    await user.click(screen.getByRole('button', { name: /send/i }));

    expect(await screen.findByText('Ready.')).toBeInTheDocument();
    expect(chatApi.send).toHaveBeenLastCalledWith(
      [{ role: 'user', content: 'Import February' }],
      'file-1',
      'February'
    );
    expect(chatApi.uploadFile).toHaveBeenCalledTimes(1);
  });

  it('does not show a sheet selector for a single-sheet upload', async () => {
    const user = userEvent.setup();
    vi.mocked(chatApi.uploadFile).mockResolvedValue(uploadedFile);

    render(<Chat />);

    await user.upload(
      screen.getByLabelText('Attach spreadsheet'),
      new File(['date,amount'], 'budget.xlsx')
    );
    await screen.findByText('budget.xlsx');

    expect(screen.queryByLabelText('Sheet')).not.toBeInTheDocument();
    expect(screen.getByText(/sheet sheet1/i)).toBeInTheDocument();
  });

  it('clears the conversation after confirmation', async () => {
    const user = userEvent.setup();
    vi.mocked(chatApi.send).mockResolvedValue({ reply: 'Hello.', tool_calls: [] });

    render(<Chat />);

    await user.type(screen.getByLabelText('Message'), 'hello');
    await user.click(screen.getByRole('button', { name: /send/i }));
    await screen.findByText('Hello.');

    await user.click(
      screen.getByRole('button', { name: /clear conversation/i })
    );
    const dialog = screen.getByRole('dialog');
    await user.click(within(dialog).getByRole('button', { name: 'Clear' }));

    expect(
      await screen.findByText(/ask me anything about your finances/i)
    ).toBeInTheDocument();
    expect(screen.queryByText('Hello.')).not.toBeInTheDocument();
  });

  it('shows upload failures inline', async () => {    const user = userEvent.setup();
    vi.mocked(chatApi.uploadFile).mockRejectedValue(
      new Error('file is too large')
    );

    render(<Chat />);

    await user.upload(
      screen.getByLabelText('Attach spreadsheet'),
      new File(['date,amount'], 'budget.xlsx')
    );

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'file is too large'
    );
  });

  it('shows a spinner and disables send while uploading', async () => {
    const user = userEvent.setup();
    vi.mocked(chatApi.uploadFile).mockReturnValue(
      new Promise<ChatFile>(() => undefined)
    );

    render(<Chat />);

    await user.upload(
      screen.getByLabelText('Attach spreadsheet'),
      new File(['date,amount'], 'budget.xlsx')
    );

    expect(screen.getByRole('status')).toHaveTextContent(/uploading spreadsheet/i);
    expect(screen.getByRole('button', { name: /send/i })).toBeDisabled();
  });
});
