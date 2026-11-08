import { create } from 'zustand';

export type ToastTone = 'success' | 'error' | 'info';

export interface Toast {
  id: string;
  title: string;
  description?: string;
  tone: ToastTone;
}

export interface ToastInput {
  title: string;
  description?: string;
  tone: ToastTone;
}

interface ToastState {
  toasts: Toast[];
  push: (input: ToastInput) => string;
  dismiss: (id: string) => void;
  clear: () => void;
}

let sequence = 0;

export const useToastStore = create<ToastState>()((set) => ({
  toasts: [],
  push: ({ title, description, tone }) => {
    sequence += 1;
    const id = `toast-${Date.now()}-${sequence}`;
    set((state) => ({
      toasts: [...state.toasts, { id, title, description, tone }],
    }));
    return id;
  },
  dismiss: (id) =>
    set((state) => ({
      toasts: state.toasts.filter((toast) => toast.id !== id),
    })),
  clear: () => set({ toasts: [] }),
}));
