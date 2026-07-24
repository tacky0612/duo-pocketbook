package dynamodb

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/tacky0612/duo-pocketbook/internal/application"
	"github.com/tacky0612/duo-pocketbook/internal/domain"
)

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

type closingDayItem struct {
	PK  string `dynamodbav:"PK"`
	SK  string `dynamodbav:"SK"`
	Day int    `dynamodbav:"Day"`
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

// GetClosingDay は設定済みの締め日を返す。未設定の場合は ok=false。
func (r *SettingsRepository) GetClosingDay(ctx context.Context) (domain.ClosingDay, bool, error) {
	out, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.table),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: settingsPK},
			"SK": &types.AttributeValueMemberS{Value: closingDaySK},
		},
	})
	if err != nil {
		return 0, false, err
	}
	if out.Item == nil {
		return 0, false, nil
	}
	var item closingDayItem
	if err := attributevalue.UnmarshalMap(out.Item, &item); err != nil {
		return 0, false, err
	}
	cd, err := domain.NewClosingDay(item.Day)
	if err != nil {
		return 0, false, err
	}
	return cd, true, nil
}

// SaveClosingDay は締め日を保存する。
func (r *SettingsRepository) SaveClosingDay(ctx context.Context, day domain.ClosingDay) error {
	item, err := attributevalue.MarshalMap(closingDayItem{PK: settingsPK, SK: closingDaySK, Day: day.Int()})
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
