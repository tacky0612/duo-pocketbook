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

// IncomeRepository は application.IncomeRepository（追加収入・複数件）の DynamoDB 実装。
// 単発は PK=INCOME#<month>、継続は PK=INCOME#RECURRING に入る。
type IncomeRepository struct {
	client *dynamodb.Client
	table  string
}

type incomeItem struct {
	PK          string `dynamodbav:"PK"`
	SK          string `dynamodbav:"SK"` // 収入ID
	MemberID    string `dynamodbav:"MemberID"`
	AmountYen   int64  `dynamodbav:"AmountYen"`
	Description string `dynamodbav:"Description"`
	Month       string `dynamodbav:"Month"` // YYYY-MM。継続は空文字
}

// incomePK はエンティティから格納先パーティションを決める。
func incomePK(inc domain.Income) string {
	if inc.IsRecurring() {
		return incomePKPrefix + incomeRecurring
	}
	return incomePKPrefix + inc.Month.String()
}

// incomePKForID はIDだけから格納先パーティションを決める（FindByID・Delete 用）。
func incomePKForID(id domain.IncomeID) (string, error) {
	if id.IsRecurring() {
		return incomePKPrefix + incomeRecurring, nil
	}
	month, err := id.Month()
	if err != nil {
		return "", err
	}
	return incomePKPrefix + month.String(), nil
}

func toIncome(item incomeItem) (domain.Income, error) {
	var month domain.YearMonth
	if item.Month != "" {
		ym, err := domain.ParseYearMonth(item.Month)
		if err != nil {
			return domain.Income{}, err
		}
		month = ym
	}
	return domain.NewIncome(item.SK, domain.MemberID(item.MemberID), domain.Money(item.AmountYen), item.Description, month)
}

// Save は収入を保存する。
func (r *IncomeRepository) Save(ctx context.Context, inc domain.Income) error {
	monthStr := ""
	if !inc.IsRecurring() {
		monthStr = inc.Month.String()
	}
	item, err := attributevalue.MarshalMap(incomeItem{
		PK:          incomePK(inc),
		SK:          string(inc.ID),
		MemberID:    string(inc.MemberID),
		AmountYen:   int64(inc.Amount),
		Description: inc.Description,
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

// FindByID はIDで収入を取得する。
func (r *IncomeRepository) FindByID(ctx context.Context, id domain.IncomeID) (domain.Income, error) {
	pk, err := incomePKForID(id)
	if err != nil {
		return domain.Income{}, err
	}
	out, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.table),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
			"SK": &types.AttributeValueMemberS{Value: string(id)},
		},
	})
	if err != nil {
		return domain.Income{}, err
	}
	if out.Item == nil {
		return domain.Income{}, application.ErrNotFound
	}
	var item incomeItem
	if err := attributevalue.UnmarshalMap(out.Item, &item); err != nil {
		return domain.Income{}, err
	}
	return toIncome(item)
}

// FindRecurring は毎月継続の収入をすべて返す。
func (r *IncomeRepository) FindRecurring(ctx context.Context) ([]domain.Income, error) {
	return r.queryByPK(ctx, incomePKPrefix+incomeRecurring)
}

// FindByMonth は指定精算月の単発の収入を返す。
func (r *IncomeRepository) FindByMonth(ctx context.Context, month domain.YearMonth) ([]domain.Income, error) {
	return r.queryByPK(ctx, incomePKPrefix+month.String())
}

func (r *IncomeRepository) queryByPK(ctx context.Context, pk string) ([]domain.Income, error) {
	paginator := dynamodb.NewQueryPaginator(r.client, &dynamodb.QueryInput{
		TableName:              aws.String(r.table),
		KeyConditionExpression: aws.String("PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: pk},
		},
	})
	var list []domain.Income
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, raw := range page.Items {
			var item incomeItem
			if err := attributevalue.UnmarshalMap(raw, &item); err != nil {
				return nil, err
			}
			inc, err := toIncome(item)
			if err != nil {
				return nil, err
			}
			list = append(list, inc)
		}
	}
	return list, nil
}

// Delete は収入を削除する。
func (r *IncomeRepository) Delete(ctx context.Context, id domain.IncomeID) error {
	pk, err := incomePKForID(id)
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
