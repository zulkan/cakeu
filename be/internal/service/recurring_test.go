package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"goskeleton/internal/model"
	"goskeleton/internal/service"
)

type fakeRecurringRepo struct {
	due     []model.RecurringRule
	claimed map[string]bool
	next    map[uuid.UUID]time.Time
}

func (f *fakeRecurringRepo) Create(ctx context.Context, rule model.RecurringRule) (*model.RecurringRule, error) {
	return &rule, nil
}

func (f *fakeRecurringRepo) ListByUser(ctx context.Context, userID uuid.UUID) ([]model.RecurringRule, error) {
	return f.due, nil
}

func (f *fakeRecurringRepo) GetByID(ctx context.Context, userID, id uuid.UUID) (*model.RecurringRule, error) {
	for i := range f.due {
		if f.due[i].ID == id {
			return &f.due[i], nil
		}
	}
	return nil, service.ErrRecurringNotFound
}

func (f *fakeRecurringRepo) Update(ctx context.Context, userID uuid.UUID, rule model.RecurringRule) (*model.RecurringRule, error) {
	return &rule, nil
}

func (f *fakeRecurringRepo) Delete(ctx context.Context, userID, id uuid.UUID) error {
	return nil
}

func (f *fakeRecurringRepo) ListDue(ctx context.Context, now time.Time, limit int) ([]model.RecurringRule, error) {
	return f.due, nil
}

func (f *fakeRecurringRepo) ClaimRun(ctx context.Context, ruleID uuid.UUID, periodStart time.Time) (bool, error) {
	key := ruleID.String() + "|" + periodStart.Format("2006-01-02")
	if f.claimed == nil {
		f.claimed = make(map[string]bool)
	}
	if f.claimed[key] {
		return false, nil
	}
	f.claimed[key] = true
	return true, nil
}

func (f *fakeRecurringRepo) AttachRunTransaction(ctx context.Context, ruleID uuid.UUID, periodStart time.Time, transactionID uuid.UUID) error {
	return nil
}

func (f *fakeRecurringRepo) ReleaseRun(ctx context.Context, ruleID uuid.UUID, periodStart time.Time) error {
	return nil
}

func (f *fakeRecurringRepo) AdvanceNextRun(ctx context.Context, ruleID uuid.UUID, next time.Time) error {
	if f.next == nil {
		f.next = make(map[uuid.UUID]time.Time)
	}
	f.next[ruleID] = next
	return nil
}

func TestAddMonthsClampedToLastDay(t *testing.T) {
	t.Parallel()

	if got := service.AddMonthsClamped(time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC), 1); !got.Equal(time.Date(2026, 2, 28, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected 2026-02-28, got %s", got)
	}

	if got := service.AddMonthsClamped(time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC), 1); !got.Equal(time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected leap year 2024-02-29, got %s", got)
	}
}

func TestRecurringRunDueCatchesUp(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	accountID := uuid.New()
	categoryID := uuid.New()
	rule := model.RecurringRule{
		ID:         uuid.New(),
		UserID:     userID,
		AccountID:  accountID,
		CategoryID: categoryID,
		Type:       "expense",
		Amount:     10,
		Frequency:  "daily",
		Interval:   1,
		NextRun:    time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		Active:     true,
	}

	repo := &fakeRecurringRepo{due: []model.RecurringRule{rule}}
	transactions := service.NewTransactionService(&fakeTransactionRepo{},
		&fakeAccountRepo{account: &model.Account{ID: accountID, UserID: userID, Name: "Cash"}},
		&fakeCategoryRepo{category: &model.Category{ID: categoryID, UserID: userID, Name: "Food", Type: "expense"}},
	)

	recurring := service.NewRecurringService(repo, transactions)

	posted, err := recurring.RunDue(context.Background(), time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if posted != 3 {
		t.Fatalf("expected 3 catch-up posts, got %d", posted)
	}
	if next := repo.next[rule.ID]; !next.Equal(time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected next run 2026-10-04, got %s", next)
	}
}

func TestRecurringRunDueSkipsAlreadyPostedPeriods(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	accountID := uuid.New()
	categoryID := uuid.New()
	rule := model.RecurringRule{
		ID:         uuid.New(),
		UserID:     userID,
		AccountID:  accountID,
		CategoryID: categoryID,
		Type:       "expense",
		Amount:     10,
		Frequency:  "daily",
		Interval:   1,
		NextRun:    time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		Active:     true,
	}

	repo := &fakeRecurringRepo{
		due:     []model.RecurringRule{rule},
		claimed: map[string]bool{rule.ID.String() + "|2026-10-01": true},
	}
	transactions := service.NewTransactionService(&fakeTransactionRepo{},
		&fakeAccountRepo{account: &model.Account{ID: accountID, UserID: userID, Name: "Cash"}},
		&fakeCategoryRepo{category: &model.Category{ID: categoryID, UserID: userID, Name: "Food", Type: "expense"}},
	)
	recurring := service.NewRecurringService(repo, transactions)

	posted, err := recurring.RunDue(context.Background(), time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if posted != 2 {
		t.Fatalf("expected 2 posts (one already claimed), got %d", posted)
	}
}
