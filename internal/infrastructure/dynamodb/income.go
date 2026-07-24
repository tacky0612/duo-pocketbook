package dynamodb

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/tacky0612/duo-pocketbook/internal/domain"
)

// IncomeRepository は application.IncomeRepository の DynamoDB 実装。
type IncomeRepository struct {
	client *dynamodb.Client
	table  string
}

type incomeItem struct {
	PK        string `dynamodbav:"PK"`
	SK        string `dynamodbav:"SK"`
	MemberID  string `dynamodbav:"MemberID"`
	AmountYen int64  `dynamodbav:"AmountYen"`
}

// Save は収入を保存（上書き）する。
func (r *IncomeRepository) Save(ctx context.Context, income domain.MonthlyIncome) error {
	item, err := attributevalue.MarshalMap(incomeItem{
		PK:        monthPKPrefix + income.Month.String(),
		SK:        incomeSKPrefix + string(income.MemberID),
		MemberID:  string(income.MemberID),
		AmountYen: int64(income.Amount),
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

// FindByMonth は対象月の収入を返す。
func (r *IncomeRepository) FindByMonth(ctx context.Context, month domain.YearMonth) ([]domain.MonthlyIncome, error) {
	out, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.table),
		KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :sk)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: monthPKPrefix + month.String()},
			":sk": &types.AttributeValueMemberS{Value: incomeSKPrefix},
		},
	})
	if err != nil {
		return nil, err
	}
	var incomes []domain.MonthlyIncome
	for _, raw := range out.Items {
		var item incomeItem
		if err := attributevalue.UnmarshalMap(raw, &item); err != nil {
			return nil, err
		}
		income, err := domain.NewMonthlyIncome(month, domain.MemberID(item.MemberID), domain.Money(item.AmountYen))
		if err != nil {
			return nil, err
		}
		incomes = append(incomes, income)
	}
	return incomes, nil
}
