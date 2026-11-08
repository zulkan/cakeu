import { useEffect, useState } from 'react';
import { Plus, Tags } from 'lucide-react';
import { useCategoryStore } from '../stores/categoryStore';
import { useToastStore } from '../stores/toastStore';
import type { Category, CreateCategoryRequest } from '../types';
import { PageHeader } from '../components/ui/PageHeader';
import { Button } from '../components/ui/Button';
import { EmptyState, ErrorState, LoadingState } from '../components/ui/States';
import { ConfirmDialog } from '../components/ui/ConfirmDialog';
import { CategoryFormModal } from '../components/categories/CategoryFormModal';
import { CategoryGroup } from '../components/categories/CategoryGroup';

function messageOf(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback;
}

function Categories() {
  const categories = useCategoryStore((state) => state.categories);
  const isLoading = useCategoryStore((state) => state.isLoading);
  const error = useCategoryStore((state) => state.error);
  const fetchCategories = useCategoryStore((state) => state.fetchCategories);
  const createCategory = useCategoryStore((state) => state.createCategory);
  const deleteCategory = useCategoryStore((state) => state.deleteCategory);
  const pushToast = useToastStore((state) => state.push);

  const [modalOpen, setModalOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<Category | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);

  useEffect(() => {
    void fetchCategories();
  }, [fetchCategories]);

  const handleCreate = async (data: CreateCategoryRequest) => {
    try {
      await createCategory(data);
      pushToast({ title: 'Category added', tone: 'success' });
    } catch (failure) {
      pushToast({ title: messageOf(failure, 'Unable to add category'), tone: 'error' });
      throw failure;
    }
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;

    setIsDeleting(true);

    try {
      await deleteCategory(deleteTarget.id);
      setDeleteTarget(null);
      pushToast({ title: 'Category deleted', tone: 'success' });
    } catch (deleteFailure) {
      pushToast({
        title: messageOf(deleteFailure, 'Unable to delete category'),
        tone: 'error',
      });
    } finally {
      setIsDeleting(false);
    }
  };

  const incomeCategories = categories.filter(
    (category) => category.type === 'income'
  );
  const expenseCategories = categories.filter(
    (category) => category.type === 'expense'
  );

  return (
    <div className="space-y-6">
      <PageHeader
        title="Categories"
        description="Shape the labels that make your spending readable."
        action={
          <Button onClick={() => setModalOpen(true)}>
            <Plus className="h-4 w-4" aria-hidden="true" />
            Add category
          </Button>
        }
      />

      {isLoading && categories.length === 0 ? (
        <LoadingState label="Loading categories…" />
      ) : error && categories.length === 0 ? (
        <ErrorState message={error} onRetry={() => void fetchCategories()} />
      ) : categories.length === 0 ? (
        <EmptyState
          icon={<Tags className="h-6 w-6" aria-hidden="true" />}
          title="No categories yet"
          description="Create income and expense categories to organize every transaction."
          action={
            <Button onClick={() => setModalOpen(true)}>
              <Plus className="h-4 w-4" aria-hidden="true" />
              Add category
            </Button>
          }
        />
      ) : (
        <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
          <CategoryGroup
            title="Income"
            description="Money coming in"
            categories={incomeCategories}
            tone="income"
            onDelete={setDeleteTarget}
          />
          <CategoryGroup
            title="Expense"
            description="Money going out"
            categories={expenseCategories}
            tone="expense"
            onDelete={setDeleteTarget}
          />
        </div>
      )}

      <CategoryFormModal
        open={modalOpen}
        onClose={() => setModalOpen(false)}
        onSubmit={handleCreate}
      />

      <ConfirmDialog
        open={deleteTarget !== null}
        title="Delete category?"
        description={
          deleteTarget
            ? `“${deleteTarget.name}” and every transaction in it will be permanently removed.`
            : ''
        }
        isLoading={isDeleting}
        onConfirm={() => void handleDelete()}
        onCancel={() => {
          if (isDeleting) return;
          setDeleteTarget(null);
        }}
      />
    </div>
  );
}

export default Categories;
