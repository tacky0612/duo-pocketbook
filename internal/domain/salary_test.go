package domain_test

import (
	"slices"
	"testing"

	"github.com/tacky0612/duo-pocketbook/internal/domain"
)

func TestEstimateSalaries(t *testing.T) {
	couple := testCouple(t)
	month := testMonth(t)
	prev := month.Prev()
	salary := func(ym domain.YearMonth, id domain.MemberID, amount domain.Money) domain.Salary {
		return domain.Salary{Month: ym, MemberID: id, Amount: amount}
	}

	tests := []struct {
		name          string
		current       []domain.Salary
		previous      []domain.Salary
		wantSalaries  []domain.Salary
		wantEstimated []domain.MemberID
	}{
		{
			name:          "当月が揃っていれば前月は使わない",
			current:       []domain.Salary{salary(month, husband, 100_000), salary(month, wife, 50_000)},
			previous:      []domain.Salary{salary(prev, husband, 1), salary(prev, wife, 1)},
			wantSalaries:  []domain.Salary{salary(month, husband, 100_000), salary(month, wife, 50_000)},
			wantEstimated: nil,
		},
		{
			name:          "当月が未入力なら両者とも前月実績で補う",
			previous:      []domain.Salary{salary(prev, husband, 90_000), salary(prev, wife, 40_000)},
			wantSalaries:  []domain.Salary{salary(month, husband, 90_000), salary(month, wife, 40_000)},
			wantEstimated: []domain.MemberID{husband, wife},
		},
		{
			name:          "未入力のメンバーだけを補う",
			current:       []domain.Salary{salary(month, wife, 50_000)},
			previous:      []domain.Salary{salary(prev, husband, 90_000), salary(prev, wife, 40_000)},
			wantSalaries:  []domain.Salary{salary(month, husband, 90_000), salary(month, wife, 50_000)},
			wantEstimated: []domain.MemberID{husband},
		},
		{
			name:          "前月も未入力のメンバーは補わない",
			previous:      []domain.Salary{salary(prev, wife, 40_000)},
			wantSalaries:  []domain.Salary{salary(month, wife, 40_000)},
			wantEstimated: []domain.MemberID{wife},
		},
		{
			name:          "前月以外の給与は使わない",
			previous:      []domain.Salary{salary(prev.Prev(), husband, 90_000)},
			wantSalaries:  nil,
			wantEstimated: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, estimated := domain.EstimateSalaries(month, couple, tt.current, tt.previous)
			if !slices.Equal(got, tt.wantSalaries) {
				t.Errorf("salaries = %+v, want %+v", got, tt.wantSalaries)
			}
			if !slices.Equal(estimated, tt.wantEstimated) {
				t.Errorf("estimated = %v, want %v", estimated, tt.wantEstimated)
			}
		})
	}
}
