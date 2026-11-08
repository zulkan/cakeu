import { create } from 'zustand';
import { chatApi, ChatApiError } from '../lib/chatApi';
import type {
  ChatEntry,
  ChatFile,
  ChatPendingAction,
  ChatResponse,
  ChatTokenUsage,
} from '../types/chat';

let sequence = 0;

function nextId(prefix: string): string {
  sequence += 1;
  return `${prefix}-${Date.now()}-${sequence}`;
}

function sendChat(
  content: string,
  fileId?: string,
  sheet?: string
): Promise<ChatResponse> {
  if (fileId && sheet) return chatApi.send(content, fileId, sheet);
  if (fileId) return chatApi.send(content, fileId);
  return chatApi.send(content);
}

function markLimitReached(usage: ChatTokenUsage | null): ChatTokenUsage {
  if (usage) return { ...usage, limit_reached: true };
  return { used: 0, limit: 0, remaining: 0, limit_reached: true };
}

interface ChatState {
  entries: ChatEntry[];
  tokenUsage: ChatTokenUsage | null;
  isLoading: boolean;
  isLoadingHistory: boolean;
  isConfirming: boolean;
  isClearing: boolean;
  error: string | null;
  attachedFile: ChatFile | null;
  selectedSheet: string | null;
  isUploading: boolean;
  uploadError: string | null;
  loadHistory: () => Promise<void>;
  sendMessage: (text: string) => Promise<void>;
  attachFile: (file: File) => Promise<void>;
  selectSheet: (name: string) => void;
  removeAttachment: () => void;
  confirmAction: (action: ChatPendingAction) => Promise<void>;
  cancelAction: (action: ChatPendingAction) => Promise<void>;
  clearError: () => void;
  clear: () => Promise<void>;
}

export const useChatStore = create<ChatState>()((set, get) => ({
  entries: [],
  tokenUsage: null,
  isLoading: false,
  isLoadingHistory: false,
  isConfirming: false,
  isClearing: false,
  error: null,
  attachedFile: null,
  selectedSheet: null,
  isUploading: false,
  uploadError: null,

  loadHistory: async () => {
    if (get().isLoadingHistory) return;
    set({ isLoadingHistory: true, error: null });

    try {
      const history = await chatApi.history();
      set({
        entries: history.messages.map((message) => ({
          id: message.id,
          role: message.role,
          content: message.content,
          toolCalls: [],
        })),
        tokenUsage: history.token_usage,
        isLoadingHistory: false,
      });
    } catch (failure) {
      set({
        isLoadingHistory: false,
        error:
          failure instanceof Error
            ? failure.message
            : 'Unable to load your conversation.',
      });
    }
  },

  sendMessage: async (text) => {
    const content = text.trim();
    if (!content || get().isUploading || get().tokenUsage?.limit_reached) return;

    const userEntry: ChatEntry = {
      id: nextId('user'),
      role: 'user',
      content,
      toolCalls: [],
    };

    const attached = get().attachedFile;
    const fileId = attached?.id;
    const sheet =
      attached && attached.sheets.length > 1
        ? get().selectedSheet ?? undefined
        : undefined;

    set((state) => ({
      entries: [...state.entries, userEntry],
      isLoading: true,
      error: null,
    }));

    try {
      const response = await sendChat(content, fileId, sheet);
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
        tokenUsage: response.token_usage ?? state.tokenUsage,
      }));
    } catch (failure) {
      const limitReached =
        failure instanceof ChatApiError && failure.status === 429;

      set((state) => ({
        isLoading: false,
        tokenUsage: limitReached
          ? markLimitReached(state.tokenUsage)
          : state.tokenUsage,
        error: limitReached
          ? null
          : failure instanceof Error
            ? failure.message
            : 'The assistant is unavailable right now.',
      }));
    }
  },

  attachFile: async (file) => {
    set({ isUploading: true, uploadError: null });

    try {
      const uploaded = await chatApi.uploadFile(file);
      set({
        attachedFile: uploaded,
        selectedSheet: uploaded.sheet,
        isUploading: false,
      });
    } catch (failure) {
      set({
        isUploading: false,
        uploadError:
          failure instanceof Error && failure.message
            ? failure.message
            : 'Unable to attach that file.',
      });
    }
  },

  selectSheet: (name) => set({ selectedSheet: name }),

  removeAttachment: () =>
    set({
      attachedFile: null,
      selectedSheet: null,
      isUploading: false,
      uploadError: null,
    }),

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
        tokenUsage: response.token_usage ?? state.tokenUsage,
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

  clear: async () => {
    if (get().isClearing) return;
    set({ isClearing: true, error: null });

    try {
      await chatApi.clear();
      set({
        entries: [],
        tokenUsage: null,
        error: null,
        isLoading: false,
        attachedFile: null,
        selectedSheet: null,
        isUploading: false,
        uploadError: null,
        isClearing: false,
      });
    } catch (failure) {
      set({
        isClearing: false,
        error:
          failure instanceof Error
            ? failure.message
            : 'Unable to clear the conversation.',
      });
    }
  },
}));
