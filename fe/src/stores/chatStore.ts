import { create } from 'zustand';
import { chatApi } from '../lib/chatApi';
import type { ChatEntry, ChatMessage, ChatPendingAction } from '../types/chat';

let sequence = 0;

function nextId(prefix: string): string {
  sequence += 1;
  return `${prefix}-${Date.now()}-${sequence}`;
}

function toHistory(entries: ChatEntry[]): ChatMessage[] {
  return entries.map((entry) => ({
    role: entry.role,
    content: entry.content,
  }));
}

interface ChatState {
  entries: ChatEntry[];
  isLoading: boolean;
  isConfirming: boolean;
  error: string | null;
  sendMessage: (text: string) => Promise<void>;
  confirmAction: (action: ChatPendingAction) => Promise<void>;
  cancelAction: (action: ChatPendingAction) => Promise<void>;
  clearError: () => void;
  clear: () => void;
}

export const useChatStore = create<ChatState>()((set, get) => ({
  entries: [],
  isLoading: false,
  isConfirming: false,
  error: null,

  sendMessage: async (text) => {
    const content = text.trim();
    if (!content) return;

    const userEntry: ChatEntry = {
      id: nextId('user'),
      role: 'user',
      content,
      toolCalls: [],
    };

    const history = [...get().entries, userEntry];
    set({ entries: history, isLoading: true, error: null });

    try {
      const response = await chatApi.send(toHistory(history));
      set((state) => ({
        entries: [
          ...state.entries,
          {
            id: nextId('assistant'),
            role: 'assistant',
            content: response.reply,
            toolCalls: response.tool_calls ?? [],
            pendingAction: response.pending_action,
          },
        ],
        isLoading: false,
      }));
    } catch (failure) {
      set({
        isLoading: false,
        error:
          failure instanceof Error
            ? failure.message
            : 'The assistant is unavailable right now.',
      });
    }
  },

  confirmAction: async (action) => {
    set({ isConfirming: true, error: null });

    try {
      const response = await chatApi.confirm(action.id);
      set((state) => ({
        entries: [
          ...state.entries.map((entry) =>
            entry.pendingAction?.id === action.id
              ? { ...entry, pendingAction: undefined }
              : entry
          ),
          {
            id: nextId('assistant'),
            role: 'assistant',
            content: response.reply,
            toolCalls: response.tool_calls ?? [],
          },
        ],
        isConfirming: false,
      }));
    } catch (failure) {
      set({
        isConfirming: false,
        error:
          failure instanceof Error
            ? failure.message
            : 'Unable to confirm that action.',
      });
    }
  },

  cancelAction: async (action) => {
    set({ isConfirming: true, error: null });

    try {
      await chatApi.cancel(action.id);
      set((state) => ({
        entries: state.entries.map((entry) =>
          entry.pendingAction?.id === action.id
            ? { ...entry, pendingAction: undefined }
            : entry
        ),
        isConfirming: false,
      }));
    } catch (failure) {
      set({
        isConfirming: false,
        error:
          failure instanceof Error ? failure.message : 'Unable to cancel that action.',
      });
    }
  },

  clearError: () => set({ error: null }),
  clear: () => set({ entries: [], error: null, isLoading: false }),
}));
