package dynamodb

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/tacky0612/duo-pocketbook/internal/domain"
)

// SettlementStatusRepository は application.SettlementStatusRepository の DynamoDB 実装。
// PK=MONTH#<month>, SK=STATUS に精算済みフラグを保持する。
type SettlementStatusRepository struct {
	client *dynamodb.Client
	table  string
}

type statusItem struct {
	PK      string `dynamodbav:"PK"`
	SK      string `dynamodbav:"SK"`
	Settled bool   `dynamodbav:"Settled"`
}

// IsSettled は対象月が精算済みかを返す。
func (r *SettlementStatusRepository) IsSettled(ctx context.Context, month domain.YearMonth) (bool, error) {
	out, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.table),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: monthPKPrefix + month.String()},
			"SK": &types.AttributeValueMemberS{Value: statusSK},
		},
	})
	if err != nil {
		return false, err
	}
	if out.Item == nil {
		return false, nil
	}
	var item statusItem
	if err := attributevalue.UnmarshalMap(out.Item, &item); err != nil {
		return false, err
	}
	return item.Settled, nil
}

// SetSettled は精算済みフラグを保存する。
func (r *SettlementStatusRepository) SetSettled(ctx context.Context, month domain.YearMonth, settled bool) error {
	item, err := attributevalue.MarshalMap(statusItem{
		PK:      monthPKPrefix + month.String(),
		SK:      statusSK,
		Settled: settled,
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
