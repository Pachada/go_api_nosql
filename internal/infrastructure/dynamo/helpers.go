package dynamo

import (
	"fmt"
	"sort"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// strKey builds a DynamoDB primary key map with a single string attribute.
func strKey(name, value string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		name: &types.AttributeValueMemberS{Value: value},
	}
}

// compositeKey builds a DynamoDB primary key with two string attributes (PK + SK).
func compositeKey(pkName, pkValue, skName, skValue string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		pkName: &types.AttributeValueMemberS{Value: pkValue},
		skName: &types.AttributeValueMemberS{Value: skValue},
	}
}

type updateExpr struct {
	Expr   string
	Names  map[string]string
	Values map[string]types.AttributeValue
}

// buildUpdateExpr converts a map of field->value into a DynamoDB SET expression.
// Keys are sorted to produce a deterministic expression string.
func buildUpdateExpr(updates map[string]interface{}) (updateExpr, error) {
	ue := updateExpr{
		Names:  make(map[string]string),
		Values: make(map[string]types.AttributeValue),
		Expr:   "SET ",
	}
	keys := make([]string, 0, len(updates))
	for k := range updates {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for i, k := range keys {
		nameKey := fmt.Sprintf("#f%d", i)
		valueKey := fmt.Sprintf(":v%d", i)
		ue.Names[nameKey] = k
		av, mErr := attributevalue.Marshal(updates[k])
		if mErr != nil {
			return updateExpr{}, fmt.Errorf("marshal field %s: %w", k, mErr)
		}
		ue.Values[valueKey] = av
		if i > 0 {
			ue.Expr += ", "
		}
		ue.Expr += fmt.Sprintf("%s = %s", nameKey, valueKey)
	}
	if len(keys) == 0 {
		return updateExpr{}, fmt.Errorf("no fields to update")
	}
	return ue, nil
}
