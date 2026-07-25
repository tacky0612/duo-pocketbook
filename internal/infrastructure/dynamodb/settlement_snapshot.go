package dynamodb

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/tacky0612/duo-pocketbook/internal/domain"
)

// SettlementSnapshotRepository は application.SettlementSnapshotRepository の DynamoDB 実装。
// PK=MONTH#<month>, SK=SNAPSHOT に精算完了時点の精算内容を保持する。
type SettlementSnapshotRepository struct {
	client *dynamodb.Client
	table  string
}

// snapshotItem は精算スナップショットの永続化表現。ドメイン型（YearMonth など）は
// プリミティブへ平坦化して保存する。
type snapshotItem struct {
	PK                     string              `dynamodbav:"PK"`
	SK                     string              `dynamodbav:"SK"`
	Month                  string              `dynamodbav:"Month"`
	SettledAt              string              `dynamodbav:"SettledAt"` // RFC3339
	TotalExpenseYen        int64               `dynamodbav:"TotalExpenseYen"`
	TotalDirectTransferYen int64               `dynamodbav:"TotalDirectTransferYen"`
	Transfer               *transferSub        `dynamodbav:"Transfer,omitempty"`
	SettlementTransfer     *transferSub        `dynamodbav:"SettlementTransfer,omitempty"`
	DirectTransfer         *transferSub        `dynamodbav:"DirectTransfer,omitempty"`
	Members                []memberSub         `dynamodbav:"Members"`
	Expenses               []expenseSub        `dynamodbav:"Expenses"`
	DirectTransfers        []directTransferSub `dynamodbav:"DirectTransfers"`
}

type transferSub struct {
	From      string `dynamodbav:"From"`
	To        string `dynamodbav:"To"`
	AmountYen int64  `dynamodbav:"AmountYen"`
}

type memberSub struct {
	ID             string `dynamodbav:"ID"`
	Name           string `dynamodbav:"Name"`
	Weight         int64  `dynamodbav:"Weight"`
	IncomeYen      int64  `dynamodbav:"IncomeYen"`
	PaidExpenseYen int64  `dynamodbav:"PaidExpenseYen"`
	DisposableYen  int64  `dynamodbav:"DisposableYen"`
}

type expenseSub struct {
	PaidBy      string `dynamodbav:"PaidBy"`
	AmountYen   int64  `dynamodbav:"AmountYen"`
	Description string `dynamodbav:"Description"`
	Date        string `dynamodbav:"Date"`
	Recurring   bool   `dynamodbav:"Recurring"`
}

type directTransferSub struct {
	From        string `dynamodbav:"From"`
	To          string `dynamodbav:"To"`
	AmountYen   int64  `dynamodbav:"AmountYen"`
	Description string `dynamodbav:"Description"`
	Recurring   bool   `dynamodbav:"Recurring"`
}

func toTransferSub(t *domain.Transfer) *transferSub {
	if t == nil {
		return nil
	}
	return &transferSub{From: string(t.From), To: string(t.To), AmountYen: int64(t.Amount)}
}

func fromTransferSub(t *transferSub) *domain.Transfer {
	if t == nil {
		return nil
	}
	return &domain.Transfer{From: domain.MemberID(t.From), To: domain.MemberID(t.To), Amount: domain.Money(t.AmountYen)}
}

// Save はスナップショットを保存（上書き）する。
func (r *SettlementSnapshotRepository) Save(ctx context.Context, s domain.SettlementSnapshot) error {
	set := s.Settlement
	item := snapshotItem{
		PK:                     monthPKPrefix + set.Month.String(),
		SK:                     snapshotSK,
		Month:                  set.Month.String(),
		SettledAt:              s.SettledAt.UTC().Format(time.RFC3339),
		TotalExpenseYen:        int64(set.TotalExpense),
		TotalDirectTransferYen: int64(set.TotalDirectTransfer),
		Transfer:               toTransferSub(set.Transfer),
		SettlementTransfer:     toTransferSub(set.SettlementTransfer),
		DirectTransfer:         toTransferSub(set.DirectTransfer),
	}
	for _, m := range set.Members {
		item.Members = append(item.Members, memberSub{
			ID:             string(m.Member.ID),
			Name:           m.Member.Name,
			Weight:         m.Weight,
			IncomeYen:      int64(m.Income),
			PaidExpenseYen: int64(m.PaidExpense),
			DisposableYen:  int64(m.Disposable),
		})
	}
	for _, e := range s.Expenses {
		item.Expenses = append(item.Expenses, expenseSub{
			PaidBy:      string(e.PaidBy),
			AmountYen:   int64(e.Amount),
			Description: e.Description,
			Date:        e.Date,
			Recurring:   e.Recurring,
		})
	}
	for _, d := range s.DirectTransfers {
		item.DirectTransfers = append(item.DirectTransfers, directTransferSub{
			From:        string(d.From),
			To:          string(d.To),
			AmountYen:   int64(d.Amount),
			Description: d.Description,
			Recurring:   d.Recurring,
		})
	}
	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return err
	}
	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.table),
		Item:      av,
	})
	return err
}

// Find は対象月のスナップショットを返す。存在しない場合は ok=false を返す。
func (r *SettlementSnapshotRepository) Find(ctx context.Context, month domain.YearMonth) (domain.SettlementSnapshot, bool, error) {
	out, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.table),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: monthPKPrefix + month.String()},
			"SK": &types.AttributeValueMemberS{Value: snapshotSK},
		},
	})
	if err != nil {
		return domain.SettlementSnapshot{}, false, err
	}
	if out.Item == nil {
		return domain.SettlementSnapshot{}, false, nil
	}
	var item snapshotItem
	if err := attributevalue.UnmarshalMap(out.Item, &item); err != nil {
		return domain.SettlementSnapshot{}, false, err
	}
	snapshot, err := toSnapshot(item)
	if err != nil {
		return domain.SettlementSnapshot{}, false, err
	}
	return snapshot, true, nil
}

// Delete は対象月のスナップショットを削除する。
func (r *SettlementSnapshotRepository) Delete(ctx context.Context, month domain.YearMonth) error {
	_, err := r.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(r.table),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: monthPKPrefix + month.String()},
			"SK": &types.AttributeValueMemberS{Value: snapshotSK},
		},
	})
	return err
}

func toSnapshot(item snapshotItem) (domain.SettlementSnapshot, error) {
	month, err := domain.ParseYearMonth(item.Month)
	if err != nil {
		return domain.SettlementSnapshot{}, fmt.Errorf("スナップショットの年月のパースに失敗しました: %w", err)
	}
	settledAt, err := time.Parse(time.RFC3339, item.SettledAt)
	if err != nil {
		return domain.SettlementSnapshot{}, fmt.Errorf("精算完了日時のパースに失敗しました: %w", err)
	}
	settlement := domain.Settlement{
		Month:               month,
		TotalExpense:        domain.Money(item.TotalExpenseYen),
		TotalDirectTransfer: domain.Money(item.TotalDirectTransferYen),
		Transfer:            fromTransferSub(item.Transfer),
		SettlementTransfer:  fromTransferSub(item.SettlementTransfer),
		DirectTransfer:      fromTransferSub(item.DirectTransfer),
	}
	for i := 0; i < len(item.Members) && i < len(settlement.Members); i++ {
		m := item.Members[i]
		settlement.Members[i] = domain.MemberSettlement{
			Member:      domain.Member{ID: domain.MemberID(m.ID), Name: m.Name},
			Weight:      m.Weight,
			Income:      domain.Money(m.IncomeYen),
			PaidExpense: domain.Money(m.PaidExpenseYen),
			Disposable:  domain.Money(m.DisposableYen),
		}
	}
	snapshot := domain.SettlementSnapshot{Settlement: settlement, SettledAt: settledAt}
	for _, e := range item.Expenses {
		snapshot.Expenses = append(snapshot.Expenses, domain.SettlementExpenseItem{
			PaidBy:      domain.MemberID(e.PaidBy),
			Amount:      domain.Money(e.AmountYen),
			Description: e.Description,
			Date:        e.Date,
			Recurring:   e.Recurring,
		})
	}
	for _, d := range item.DirectTransfers {
		snapshot.DirectTransfers = append(snapshot.DirectTransfers, domain.SettlementDirectTransferItem{
			From:        domain.MemberID(d.From),
			To:          domain.MemberID(d.To),
			Amount:      domain.Money(d.AmountYen),
			Description: d.Description,
			Recurring:   d.Recurring,
		})
	}
	return snapshot, nil
}
