import { describe, it, expect, beforeEach, vi } from 'vitest';
import { api } from '../../lib/api';
import { useRecurringStore } from '../recurringStore';
import type { CreateRecurringRequest, RecurringRule } from '../../types';

vi.mock('../../lib/api', () => ({
  api: {
    listRecurring: vi.fn(),
    createRecurring: vi.fn(),
    updateRecurring: vi.fn(),
    deleteRecurring: vi.fn(),
    runRecurringNow: vi.fn(),
  },
}));

const rule: RecurringRule = {
  id: 'rule-1',
  user_id: 'user-1',
  account_id: 'account-1',
  category_id: 'category-1',
  type: 'expense',
  amount: 1200,
  description: 'Rent',
  frequency: 'monthly',
  interval: 1,
  next_run: '2026-12-01T00:00:00Z',
  active: true,
  created_at: '2026-11-01T00:00:00Z',
  updated_at: '2026-11-01T00:00:00Z',
};

const createPayload: CreateRecurringRequest = {
  account_id: 'account-1',
  category_id: 'category-1',
  type: 'expense',
  amount: 1200,
  description: 'Rent',
  frequency: 'monthly',
  interval: 1,
  next_run: '2026-12-01',
};

describe('recurring store', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useRecurringStore.setState({
      rules: [],
      isLoading: false,
      error: null,
    });
  });

  it('loads recurring rules', async () => {
    vi.mocked(api.listRecurring).mockResolvedValue([rule]);

    await useRecurringStore.getState().fetchRules();

    expect(useRecurringStore.getState().rules).toEqual([rule]);
    expect(useRecurringStore.getState().isLoading).toBe(false);
  });

  it('captures a load error', async () => {
    vi.mocked(api.listRecurring).mockRejectedValue(new Error('boom'));

    await useRecurringStore.getState().fetchRules();

    expect(useRecurringStore.getState().error).toBe('boom');
    expect(useRecurringStore.getState().isLoading).toBe(false);
  });

  it('appends a created rule', async () => {
    vi.mocked(api.createRecurring).mockResolvedValue(rule);

    const created = await useRecurringStore.getState().createRule(createPayload);

    expect(api.createRecurring).toHaveBeenCalledWith(createPayload);
    expect(created).toEqual(rule);
    expect(useRecurringStore.getState().rules).toEqual([rule]);
  });

  it('replaces an updated rule', async () => {
    useRecurringStore.setState({ rules: [rule] });
    const paused = { ...rule, active: false };
    vi.mocked(api.updateRecurring).mockResolvedValue(paused);

    await useRecurringStore.getState().updateRule(rule.id, { active: false });

    expect(api.updateRecurring).toHaveBeenCalledWith(rule.id, {
      active: false,
    });
    expect(useRecurringStore.getState().rules).toEqual([paused]);
  });

  it('removes a deleted rule', async () => {
    useRecurringStore.setState({ rules: [rule] });
    vi.mocked(api.deleteRecurring).mockResolvedValue(undefined);

    await useRecurringStore.getState().deleteRule(rule.id);

    expect(api.deleteRecurring).toHaveBeenCalledWith(rule.id);
    expect(useRecurringStore.getState().rules).toEqual([]);
  });

  it('runs a rule now, returns the posted count, and refreshes', async () => {
    const advanced = {
      ...rule,
      next_run: '2027-01-01T00:00:00Z',
    };
    vi.mocked(api.runRecurringNow).mockResolvedValue({ posted: 2 });
    vi.mocked(api.listRecurring).mockResolvedValue([advanced]);

    const posted = await useRecurringStore.getState().runNow(rule.id);

    expect(posted).toBe(2);
    expect(api.runRecurringNow).toHaveBeenCalledWith(rule.id);
    expect(api.listRecurring).toHaveBeenCalled();
    expect(useRecurringStore.getState().rules).toEqual([advanced]);
  });
});
