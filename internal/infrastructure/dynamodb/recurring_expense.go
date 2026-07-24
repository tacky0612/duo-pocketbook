package dynamodb

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/tacky0612/duo-pocketbook/internal/application"
	"github.com/tacky0612/duo-pocketbook/internal/domain"
)

// RecurringExpenseRepository は application.RecurringExpenseRepository の DynamoDB 実装。
// 全レコードが単一パーティション (PK=RECURRING) に入る。
type RecurringExpenseRepository struct {
	client *dynamodb.Client
	table  string
}

type recurringItem struct {
	PK          string `dynamodbav:"PK"`
	SK          string `dynamodbav:"SK"` // 固定費ID
	PaidBy      string `dynamodbav:"PaidBy"`
	AmountYen   int64  `dynamodbav:"AmountYen"`
	Description string `dynamodbav:"Description"`
}

func recurringKey(id domain.RecurringExpenseID) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"PK": &types.AttributeValueMemberS{Value: recurringPK},
		"SK": &types.AttributeValueMemberS{Value: string(id)},
	}
}

func toRecurring(item recurringItem) (domain.RecurringExpense, error) {
	return domain.NewRecurringExpense(item.SK, domain.MemberID(item.PaidBy), domain.Money(item.AmountYen), item.Description)
}

// Save は固定費を保存する。
func (r *RecurringExpenseRepository) Save(ctx context.Context, e domain.RecurringExpense) error {
	item, err := attributevalue.MarshalMap(recurringItem{
		PK:          recurringPK,
		SK:          string(e.ID),
		PaidBy:      string(e.PaidBy),
		AmountYen:   int64(e.Amount),
		Description: e.Description,
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

// FindByID はIDで固定費を取得する。
func (r *RecurringExpenseRepository) FindByID(ctx context.Context, id domain.RecurringExpenseID) (domain.RecurringExpense, error) {
	out, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.table),
		Key:       recurringKey(id),
	})
	if err != nil {
		return domain.RecurringExpense{}, err
	}
	if out.Item == nil {
		return domain.RecurringExpense{}, application.ErrNotFound
	}
	var item recurringItem
	if err := attributevalue.UnmarshalMap(out.Item, &item); err != nil {
		return domain.RecurringExpense{}, err
	}
	return toRecurring(item)
}

// FindAll は全固定費を返す。
func (r *RecurringExpenseRepository) FindAll(ctx context.Context) ([]domain.RecurringExpense, error) {
	paginator := dynamodb.NewQueryPaginator(r.client, &dynamodb.QueryInput{
		TableName:              aws.String(r.table),
		KeyConditionExpression: aws.String("PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: recurringPK},
		},
	})
	var list []domain.RecurringExpense
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, raw := range page.Items {
			var item recurringItem
			if err := attributevalue.UnmarshalMap(raw, &item); err != nil {
				return nil, err
			}
			e, err := toRecurring(item)
			if err != nil {
				return nil, err
			}
			list = append(list, e)
		}
	}
	return list, nil
}

// Delete は固定費を削除する。
func (r *RecurringExpenseRepository) Delete(ctx context.Context, id domain.RecurringExpenseID) error {
	_, err := r.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(r.table),
		Key:       recurringKey(id),
	})
	return err
}
