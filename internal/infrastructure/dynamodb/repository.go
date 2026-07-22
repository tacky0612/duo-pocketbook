package dynamodb

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/tacky0612/duo-pocketbook/internal/application"
	"github.com/tacky0612/duo-pocketbook/internal/domain"
)

const (
	expensePKPrefix = "EXPENSE#"
	monthPKPrefix   = "MONTH#"
	incomeSKPrefix  = "INCOME#"
	settingsPK      = "SETTINGS"
	weightSK        = "WEIGHT"
	profileSKPrefix = "PROFILE#"
	recurringPK     = "RECURRING"
	statusSK        = "STATUS"
)

// Repositories は DynamoDB 実装のリポジトリ群。
type Repositories struct {
	Expenses  *ExpenseRepository
	Incomes   *IncomeRepository
	Recurring *RecurringExpenseRepository
	Settings  *SettingsRepository
	Status    *SettlementStatusRepository
}

// NewRepositories は同一テーブルを共有するリポジトリ群を生成する。
func NewRepositories(client *dynamodb.Client, tableName string) Repositories {
	return Repositories{
		Expenses:  &ExpenseRepository{client: client, table: tableName},
		Incomes:   &IncomeRepository{client: client, table: tableName},
		Recurring: &RecurringExpenseRepository{client: client, table: tableName},
		Settings:  &SettingsRepository{client: client, table: tableName},
		Status:    &SettlementStatusRepository{client: client, table: tableName},
	}
}

// ---- 支出 ----

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
	}, nil
}

// Save は支出を保存する。
func (r *ExpenseRepository) Save(ctx context.Context, e domain.Expense) error {
	item, err := attributevalue.MarshalMap(expenseItem{
		PK:          expensePKPrefix + e.Month().String(),
		SK:          string(e.ID),
		PaidBy:      string(e.PaidBy),
		AmountYen:   int64(e.Amount),
		Description: e.Description,
		Date:        e.Date.Format("2006-01-02"),
		CreatedAt:   e.CreatedAt.UTC().Format(time.RFC3339),
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
	key, err := expenseKey(id)
	if err != nil {
		return domain.Expense{}, err
	}
	out, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.table),
		Key:       key,
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
	paginator := dynamodb.NewQueryPaginator(r.client, &dynamodb.QueryInput{
		TableName:              aws.String(r.table),
		KeyConditionExpression: aws.String("PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: expensePKPrefix + month.String()},
		},
	})
	var expenses []domain.Expense
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, raw := range page.Items {
			var item expenseItem
			if err := attributevalue.UnmarshalMap(raw, &item); err != nil {
				return nil, err
			}
			e, err := toExpense(item)
			if err != nil {
				return nil, err
			}
			expenses = append(expenses, e)
		}
	}
	return expenses, nil
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

// ---- 収入 ----

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

// ---- 精算ステータス ----

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

// ---- 固定費 ----

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

// ---- 設定 ----

// SettingsRepository は application.SettingsRepository の DynamoDB 実装。
type SettingsRepository struct {
	client *dynamodb.Client
	table  string
}

type weightItem struct {
	PK      string           `dynamodbav:"PK"`
	SK      string           `dynamodbav:"SK"`
	Weights map[string]int64 `dynamodbav:"Weights"`
}

type profileItem struct {
	PK       string `dynamodbav:"PK"`
	SK       string `dynamodbav:"SK"` // PROFILE#<memberID>
	MemberID string `dynamodbav:"MemberID"`
	Name     string `dynamodbav:"Name"`
	Color    string `dynamodbav:"Color"`
}

// GetWeight は設定済みの比重を返す。未設定の場合は ok=false。
func (r *SettingsRepository) GetWeight(ctx context.Context) (domain.Weight, bool, error) {
	out, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.table),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: settingsPK},
			"SK": &types.AttributeValueMemberS{Value: weightSK},
		},
	})
	if err != nil {
		return domain.Weight{}, false, err
	}
	if out.Item == nil {
		return domain.Weight{}, false, nil
	}
	var item weightItem
	if err := attributevalue.UnmarshalMap(out.Item, &item); err != nil {
		return domain.Weight{}, false, err
	}
	ids := make([]domain.MemberID, 0, len(item.Weights))
	values := make([]int64, 0, len(item.Weights))
	for id, v := range item.Weights {
		ids = append(ids, domain.MemberID(id))
		values = append(values, v)
	}
	if len(ids) != 2 {
		return domain.Weight{}, false, fmt.Errorf("比重データが不正です: %v", item.Weights)
	}
	weight, err := domain.NewWeight(ids[0], values[0], ids[1], values[1])
	if err != nil {
		return domain.Weight{}, false, err
	}
	return weight, true, nil
}

// SaveWeight は比重を保存する。
func (r *SettingsRepository) SaveWeight(ctx context.Context, weight domain.Weight) error {
	weights := map[string]int64{}
	for id, v := range weight.Entries() {
		weights[string(id)] = v
	}
	item, err := attributevalue.MarshalMap(weightItem{PK: settingsPK, SK: weightSK, Weights: weights})
	if err != nil {
		return err
	}
	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.table),
		Item:      item,
	})
	return err
}

// GetMemberProfiles はプロフィールの上書き設定を返す。
func (r *SettingsRepository) GetMemberProfiles(ctx context.Context) (map[domain.MemberID]application.MemberProfile, error) {
	out, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.table),
		KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :sk)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: settingsPK},
			":sk": &types.AttributeValueMemberS{Value: profileSKPrefix},
		},
	})
	if err != nil {
		return nil, err
	}
	profiles := map[domain.MemberID]application.MemberProfile{}
	for _, raw := range out.Items {
		var item profileItem
		if err := attributevalue.UnmarshalMap(raw, &item); err != nil {
			return nil, err
		}
		// メンバーIDはSKからも導出できるが、属性があればそれを優先する。
		id := item.MemberID
		if id == "" {
			id = strings.TrimPrefix(item.SK, profileSKPrefix)
		}
		profiles[domain.MemberID(id)] = application.MemberProfile{Name: item.Name, Color: item.Color}
	}
	return profiles, nil
}

// SaveMemberName は表示名を保存する（他の属性は維持するため UpdateItem を使う）。
func (r *SettingsRepository) SaveMemberName(ctx context.Context, id domain.MemberID, name string) error {
	return r.updateProfileField(ctx, id, "Name", name)
}

// SaveMemberColor はカラーを保存する（他の属性は維持する）。
func (r *SettingsRepository) SaveMemberColor(ctx context.Context, id domain.MemberID, color string) error {
	return r.updateProfileField(ctx, id, "Color", color)
}

// updateProfileField はプロフィールの単一属性のみを更新する。
func (r *SettingsRepository) updateProfileField(ctx context.Context, id domain.MemberID, attr, value string) error {
	_, err := r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.table),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: settingsPK},
			"SK": &types.AttributeValueMemberS{Value: profileSKPrefix + string(id)},
		},
		UpdateExpression: aws.String("SET #field = :v, MemberID = :mid"),
		ExpressionAttributeNames: map[string]string{
			"#field": attr,
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":v":   &types.AttributeValueMemberS{Value: value},
			":mid": &types.AttributeValueMemberS{Value: string(id)},
		},
	})
	return err
}
