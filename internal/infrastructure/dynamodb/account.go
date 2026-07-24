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

// AccountRepository は application.AccountRepository の DynamoDB 実装。
// 全レコードが単一パーティション (PK=ACCOUNT) に入る。
type AccountRepository struct {
	client *dynamodb.Client
	table  string
}

type accountItem struct {
	PK           string `dynamodbav:"PK"`
	SK           string `dynamodbav:"SK"` // ACCT#<accountId>
	AccountID    string `dynamodbav:"AccountID"`
	Slot         int    `dynamodbav:"Slot"`
	LoginID      string `dynamodbav:"LoginID"`
	PasswordHash string `dynamodbav:"PasswordHash"`
}

// List は全アカウントを返す。
func (r *AccountRepository) List(ctx context.Context) ([]application.Account, error) {
	paginator := dynamodb.NewQueryPaginator(r.client, &dynamodb.QueryInput{
		TableName:              aws.String(r.table),
		KeyConditionExpression: aws.String("PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: accountPK},
		},
	})
	var list []application.Account
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, raw := range page.Items {
			var item accountItem
			if err := attributevalue.UnmarshalMap(raw, &item); err != nil {
				return nil, err
			}
			list = append(list, application.Account{
				ID:           domain.MemberID(item.AccountID),
				Slot:         item.Slot,
				LoginID:      item.LoginID,
				PasswordHash: item.PasswordHash,
			})
		}
	}
	return list, nil
}

// Save はアカウントを保存（upsert）する。
func (r *AccountRepository) Save(ctx context.Context, a application.Account) error {
	item, err := attributevalue.MarshalMap(accountItem{
		PK:           accountPK,
		SK:           accountSKPrefix + string(a.ID),
		AccountID:    string(a.ID),
		Slot:         a.Slot,
		LoginID:      a.LoginID,
		PasswordHash: a.PasswordHash,
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
