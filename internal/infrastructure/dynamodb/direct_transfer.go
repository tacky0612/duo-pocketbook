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

// DirectTransferRepository は application.DirectTransferRepository の DynamoDB 実装。
// 単発は PK=DIRECTTRANSFER#<month>、継続は PK=DIRECTTRANSFER#RECURRING に入る。
type DirectTransferRepository struct {
	client *dynamodb.Client
	table  string
}

type directTransferItem struct {
	PK          string `dynamodbav:"PK"`
	SK          string `dynamodbav:"SK"` // 立替精算ID
	FromID      string `dynamodbav:"FromID"`
	ToID        string `dynamodbav:"ToID"`
	AmountYen   int64  `dynamodbav:"AmountYen"`
	Description string `dynamodbav:"Description"`
	Month       string `dynamodbav:"Month"` // YYYY-MM。継続は空文字
}

// directTransferPK はエンティティから格納先パーティションを決める。
func directTransferPK(dt domain.DirectTransfer) string {
	if dt.IsRecurring() {
		return directPKPrefix + directRecurring
	}
	return directPKPrefix + dt.Month.String()
}

// directTransferPKForID はIDだけから格納先パーティションを決める（FindByID・Delete 用）。
func directTransferPKForID(id domain.DirectTransferID) (string, error) {
	if id.IsRecurring() {
		return directPKPrefix + directRecurring, nil
	}
	month, err := id.Month()
	if err != nil {
		return "", err
	}
	return directPKPrefix + month.String(), nil
}

func toDirectTransfer(item directTransferItem) (domain.DirectTransfer, error) {
	var month domain.YearMonth
	if item.Month != "" {
		ym, err := domain.ParseYearMonth(item.Month)
		if err != nil {
			return domain.DirectTransfer{}, err
		}
		month = ym
	}
	return domain.NewDirectTransfer(item.SK, domain.MemberID(item.FromID), domain.MemberID(item.ToID), domain.Money(item.AmountYen), item.Description, month)
}

// Save は立替精算を保存する。
func (r *DirectTransferRepository) Save(ctx context.Context, dt domain.DirectTransfer) error {
	monthStr := ""
	if !dt.IsRecurring() {
		monthStr = dt.Month.String()
	}
	item, err := attributevalue.MarshalMap(directTransferItem{
		PK:          directTransferPK(dt),
		SK:          string(dt.ID),
		FromID:      string(dt.From),
		ToID:        string(dt.To),
		AmountYen:   int64(dt.Amount),
		Description: dt.Description,
		Month:       monthStr,
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

// FindByID はIDで立替精算を取得する。
func (r *DirectTransferRepository) FindByID(ctx context.Context, id domain.DirectTransferID) (domain.DirectTransfer, error) {
	pk, err := directTransferPKForID(id)
	if err != nil {
		return domain.DirectTransfer{}, err
	}
	out, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.table),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
			"SK": &types.AttributeValueMemberS{Value: string(id)},
		},
	})
	if err != nil {
		return domain.DirectTransfer{}, err
	}
	if out.Item == nil {
		return domain.DirectTransfer{}, application.ErrNotFound
	}
	var item directTransferItem
	if err := attributevalue.UnmarshalMap(out.Item, &item); err != nil {
		return domain.DirectTransfer{}, err
	}
	return toDirectTransfer(item)
}

// FindRecurring は毎月継続の立替精算をすべて返す。
func (r *DirectTransferRepository) FindRecurring(ctx context.Context) ([]domain.DirectTransfer, error) {
	return r.queryByPK(ctx, directPKPrefix+directRecurring)
}

// FindByMonth は指定精算月の単発の立替精算を返す。
func (r *DirectTransferRepository) FindByMonth(ctx context.Context, month domain.YearMonth) ([]domain.DirectTransfer, error) {
	return r.queryByPK(ctx, directPKPrefix+month.String())
}

func (r *DirectTransferRepository) queryByPK(ctx context.Context, pk string) ([]domain.DirectTransfer, error) {
	paginator := dynamodb.NewQueryPaginator(r.client, &dynamodb.QueryInput{
		TableName:              aws.String(r.table),
		KeyConditionExpression: aws.String("PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: pk},
		},
	})
	var list []domain.DirectTransfer
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, raw := range page.Items {
			var item directTransferItem
			if err := attributevalue.UnmarshalMap(raw, &item); err != nil {
				return nil, err
			}
			dt, err := toDirectTransfer(item)
			if err != nil {
				return nil, err
			}
			list = append(list, dt)
		}
	}
	return list, nil
}

// Delete は立替精算を削除する。
func (r *DirectTransferRepository) Delete(ctx context.Context, id domain.DirectTransferID) error {
	pk, err := directTransferPKForID(id)
	if err != nil {
		return err
	}
	_, err = r.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(r.table),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
			"SK": &types.AttributeValueMemberS{Value: string(id)},
		},
	})
	return err
}
