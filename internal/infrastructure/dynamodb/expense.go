package dynamodb

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/tacky0612/duo-pocketbook/internal/application"
	"github.com/tacky0612/duo-pocketbook/internal/domain"
)

// ExpenseRepository は application.ExpenseRepository の DynamoDB 実装。
type ExpenseRepository struct {
	client *dynamodb.Client
	table  string
}

type expenseItem struct {
	PK          string `dynamodbav:"PK"`
	SK          string `dynamodbav:"SK"`
	PaidBy      string `dynamodbav:"PaidBy"`
	AmountYen   int64  `dynamodbav:"AmountYen"`
	Description string `dynamodbav:"Description"`
	Date        string `dynamodbav:"Date"`      // YYYY-MM-DD
	CreatedAt   string `dynamodbav:"CreatedAt"` // RFC3339
	// ReservationID は予約から登録された支出のみ持つ（通常の支出は属性なし）。
	ReservationID string `dynamodbav:"ReservationID,omitempty"`
}

func expenseKey(id domain.ExpenseID) (map[string]types.AttributeValue, error) {
	month, err := id.Month()
	if err != nil {
		return nil, err
	}
	return map[string]types.AttributeValue{
		"PK": &types.AttributeValueMemberS{Value: expensePKPrefix + month.String()},
		"SK": &types.AttributeValueMemberS{Value: string(id)},
	}, nil
}

func toExpense(item expenseItem) (domain.Expense, error) {
	date, err := time.Parse("2006-01-02", item.Date)
	if err != nil {
		return domain.Expense{}, fmt.Errorf("支出日のパースに失敗しました: %w", err)
	}
	createdAt, err := time.Parse(time.RFC3339, item.CreatedAt)
	if err != nil {
		return domain.Expense{}, fmt.Errorf("作成日時のパースに失敗しました: %w", err)
	}
	return domain.Expense{
		ID:          domain.ExpenseID(item.SK),
		PaidBy:      domain.MemberID(item.PaidBy),
		Amount:      domain.Money(item.AmountYen),
		Description: item.Description,
		Date:        date,
		CreatedAt:   createdAt,
		Reservation: domain.NewReservationRef(domain.ReservationID(item.ReservationID)),
	}, nil
}

// Save は支出を保存する。
func (r *ExpenseRepository) Save(ctx context.Context, e domain.Expense) error {
	item, err := attributevalue.MarshalMap(expenseItem{
		PK:            expensePKPrefix + e.Month().String(),
		SK:            string(e.ID),
		PaidBy:        string(e.PaidBy),
		AmountYen:     int64(e.Amount),
		Description:   e.Description,
		Date:          e.Date.Format("2006-01-02"),
		CreatedAt:     e.CreatedAt.UTC().Format(time.RFC3339),
		ReservationID: e.Reservation.String(),
	})
	if err != nil {
		return err
	}
	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.table),
		Item:      item,
	})
	return err
}

// FindByID はIDで支出を取得する。
func (r *ExpenseRepository) FindByID(ctx context.Context, id domain.ExpenseID) (domain.Expense, error) {
	return r.findByID(ctx, id, false)
}

// FindByIDConsistent は強い整合性の GetItem で支出を取得する（読み取りコストは2倍）。
func (r *ExpenseRepository) FindByIDConsistent(ctx context.Context, id domain.ExpenseID) (domain.Expense, error) {
	return r.findByID(ctx, id, true)
}

func (r *ExpenseRepository) findByID(ctx context.Context, id domain.ExpenseID, consistent bool) (domain.Expense, error) {
	key, err := expenseKey(id)
	if err != nil {
		return domain.Expense{}, err
	}
	out, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName:      aws.String(r.table),
		Key:            key,
		ConsistentRead: aws.Bool(consistent),
	})
	if err != nil {
		return domain.Expense{}, err
	}
	if out.Item == nil {
		return domain.Expense{}, application.ErrNotFound
	}
	var item expenseItem
	if err := attributevalue.UnmarshalMap(out.Item, &item); err != nil {
		return domain.Expense{}, err
	}
	return toExpense(item)
}

// FindByMonth は対象月の支出を返す。
func (r *ExpenseRepository) FindByMonth(ctx context.Context, month domain.YearMonth) ([]domain.Expense, error) {
	return queryByPK(ctx, r.client, r.table, expensePKPrefix+month.String(), toExpense)
}

// Delete は支出を削除する。
func (r *ExpenseRepository) Delete(ctx context.Context, id domain.ExpenseID) error {
	key, err := expenseKey(id)
	if err != nil {
		return err
	}
	_, err = r.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(r.table),
		Key:       key,
	})
	return err
}
