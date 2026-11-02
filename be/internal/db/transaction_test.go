package db

import (
	"testing"

	"github.com/google/uuid"

	"goskeleton/internal/service"
)

func TestBalanceDeltas(t *testing.T) {
	t.Parallel()

	accountA := uuid.New()
	accountB := uuid.New()

	tests := []struct {
		name       string
		oldAccount uuid.UUID
		oldType    string
		oldAmount  float64
		newAccount uuid.UUID
		newType    string
		newAmount  float64
		want       map[uuid.UUID]float64
	}{
		{
			name:       "expense amount increased on same account",
			oldAccount: accountA,
			oldType:    service.TypeExpense,
			oldAmount:  30,
			newAccount: accountA,
			newType:    service.TypeExpense,
			newAmount:  50,
			want:       map[uuid.UUID]float64{accountA: -20},
		},
		{
			name:       "unchanged transaction",
			oldAccount: accountA,
			oldType:    service.TypeExpense,
			oldAmount:  30,
			newAccount: accountA,
			newType:    service.TypeExpense,
			newAmount:  30,
			want:       map[uuid.UUID]float64{accountA: 0},
		},
		{
			name:       "expense becomes income on same account",
			oldAccount: accountA,
			oldType:    service.TypeExpense,
			oldAmount:  30,
			newAccount: accountA,
			newType:    service.TypeIncome,
			newAmount:  50,
			want:       map[uuid.UUID]float64{accountA: 80},
		},
		{
			name:       "expense moves to another account",
			oldAccount: accountA,
			oldType:    service.TypeExpense,
			oldAmount:  30,
			newAccount: accountB,
			newType:    service.TypeExpense,
			newAmount:  30,
			want:       map[uuid.UUID]float64{accountA: 30, accountB: -30},
		},
		{
			name:       "income moves to another account",
			oldAccount: accountA,
			oldType:    service.TypeIncome,
			oldAmount:  30,
			newAccount: accountB,
			newType:    service.TypeIncome,
			newAmount:  30,
			want:       map[uuid.UUID]float64{accountA: -30, accountB: 30},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := balanceDeltas(
				test.oldAccount,
				test.oldType,
				test.oldAmount,
				test.newAccount,
				test.newType,
				test.newAmount,
			)

			if len(got) != len(test.want) {
				t.Fatalf("expected %d accounts, got %d", len(test.want), len(got))
			}
			for accountID, want := range test.want {
				if got[accountID] != want {
					t.Fatalf("expected delta %v for account %s, got %v", want, accountID, got[accountID])
				}
			}
		})
	}
}
