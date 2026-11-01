import { create } from 'zustand';
import { chatApi } from '../lib/chatApi';
import type {
  ChatEntry,
  ChatFile,
  ChatMessage,
  ChatPendingAction,
  ChatResponse,
} from '../types/chat';

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

function sendHistory(
  history: ChatMessage[],
  fileId?: string,
  sheet?: string
): Promise<ChatResponse> {
  if (fileId && sheet) return chatApi.send(history, fileId, sheet);
  if (fileId) return chatApi.send(history, fileId);
  return chatApi.send(history);
}

interface ChatState {
  entries: ChatEntry[];
  isLoading: boolean;
  isConfirming: boolean;
  error: string | null;
  attachedFile: ChatFile | null;
  selectedSheet: string | null;
  isUploading: boolean;
  uploadError: string | null;
  sendMessage: (text: string) => Promise<void>;
  attachFile: (file: File) => Promise<void>;
  selectSheet: (name: string) => void;
  removeAttachment: () => void;
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
  attachedFile: null,
  selectedSheet: null,
  isUploading: false,
  uploadError: null,

  sendMessage: async (text) => {
    const content = text.trim();
    if (!content || get().isUploading) return;

    const userEntry: ChatEntry = {
      id: nextId('user'),
      role: 'user',
      content,
      toolCalls: [],
    };

    const history = [...get().entries, userEntry];
    const attached = get().attachedFile;
    const fileId = attached?.id;
    const sheet =
      attached && attached.sheets.length > 1
        ? get().selectedSheet ?? undefined
        : undefined;

    set({ entries: history, isLoading: true, error: null });

    try {
      const response = await sendHistory(toHistory(history), fileId, sheet);
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
  clear: () =>
    set({
      entries: [],
      error: null,
      isLoading: false,
      attachedFile: null,
      selectedSheet: null,
      isUploading: false,
      uploadError: null,
    }),
}));
