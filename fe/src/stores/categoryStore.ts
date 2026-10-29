import { create } from 'zustand';
import type { Category, CreateCategoryRequest } from '../types';
import { api } from '../lib/api';

interface CategoryState {
  categories: Category[];
  isLoading: boolean;
  error: string | null;
  fetchCategories: () => Promise<void>;
  createCategory: (data: CreateCategoryRequest) => Promise<Category>;
  deleteCategory: (id: string) => Promise<void>;
}

export const useCategoryStore = create<CategoryState>()((set) => ({
  categories: [],
  isLoading: false,
  error: null,
  fetchCategories: async () => {
    set({ isLoading: true, error: null });
    try {
      const categories = await api.listCategories();
      set({ categories, isLoading: false });
    } catch (e) {
      set({ error: (e as Error).message, isLoading: false });
    }
  },
  createCategory: async (data) => {
    const category = await api.createCategory(data);
    set((s) => ({ categories: [...s.categories, category] }));
    return category;
  },
  deleteCategory: async (id) => {
    await api.deleteCategory(id);
    set((s) => ({ categories: s.categories.filter((c) => c.id !== id) }));
  },
}));
