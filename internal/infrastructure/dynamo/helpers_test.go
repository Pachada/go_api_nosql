package dynamo

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildUpdateExpr_SingleField(t *testing.T) {
	ue, err := buildUpdateExpr(map[string]interface{}{"username": "alice"})
	require.NoError(t, err)
	assert.Equal(t, "SET #f0 = :v0", ue.Expr)
	assert.Equal(t, map[string]string{"#f0": "username"}, ue.Names)
	_, ok := ue.Values[":v0"]
	assert.True(t, ok)
}

func TestBuildUpdateExpr_MultipleFields_Deterministic(t *testing.T) {
	updates := map[string]interface{}{
		"email":      "a@b.com",
		"first_name": "Alice",
		"username":   "alice",
	}
	// Call twice to verify determinism.
	ue1, err := buildUpdateExpr(updates)
	require.NoError(t, err)
	ue2, err := buildUpdateExpr(updates)
	require.NoError(t, err)

	assert.Equal(t, ue1.Expr, ue2.Expr)

	// Keys must be sorted: email < first_name < username
	assert.Equal(t, "email", ue1.Names["#f0"])
	assert.Equal(t, "first_name", ue1.Names["#f1"])
	assert.Equal(t, "username", ue1.Names["#f2"])
	assert.Equal(t, "SET #f0 = :v0, #f1 = :v1, #f2 = :v2", ue1.Expr)
}

func TestBuildUpdateExpr_ValuesMarshalledCorrectly(t *testing.T) {
	ue, err := buildUpdateExpr(map[string]interface{}{"enable": true})
	require.NoError(t, err)
	av, ok := ue.Values[":v0"]
	require.True(t, ok)
	boolVal, isBool := av.(*types.AttributeValueMemberBOOL)
	require.True(t, isBool)
	assert.True(t, boolVal.Value)
}

func TestBuildUpdateExpr_EmptyMap_ReturnsError(t *testing.T) {
	_, err := buildUpdateExpr(map[string]interface{}{})
	assert.ErrorContains(t, err, "no fields to update")
}
