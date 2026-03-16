package check

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Step 5.1: Tokenizer ---

func TestTokenizeNumber(t *testing.T) {
	tokens, err := tokenize("42")
	require.NoError(t, err)
	require.Len(t, tokens, 2) // NUMBER + EOF
	assert.Equal(t, tokNumber, tokens[0].typ)
	assert.Equal(t, "42", tokens[0].val)
}

func TestTokenizeFloat(t *testing.T) {
	tokens, err := tokenize("3.14")
	require.NoError(t, err)
	assert.Equal(t, tokNumber, tokens[0].typ)
	assert.Equal(t, "3.14", tokens[0].val)
}

func TestTokenizeIdentifier(t *testing.T) {
	tokens, err := tokenize("upstream_rate")
	require.NoError(t, err)
	assert.Equal(t, tokIdent, tokens[0].typ)
	assert.Equal(t, "upstream_rate", tokens[0].val)
}

func TestTokenizeOperators(t *testing.T) {
	tokens, err := tokenize("+ - * / <= >= < >")
	require.NoError(t, err)
	expected := []tokenType{tokPlus, tokMinus, tokStar, tokSlash, tokLTE, tokGTE, tokLT, tokGT, tokEOF}
	require.Len(t, tokens, len(expected))
	for i, exp := range expected {
		assert.Equal(t, exp, tokens[i].typ, "token %d", i)
	}
}

func TestTokenizeParens(t *testing.T) {
	tokens, err := tokenize("(a + b)")
	require.NoError(t, err)
	expected := []tokenType{tokLParen, tokIdent, tokPlus, tokIdent, tokRParen, tokEOF}
	require.Len(t, tokens, len(expected))
	for i, exp := range expected {
		assert.Equal(t, exp, tokens[i].typ, "token %d", i)
	}
}

func TestTokenizeComma(t *testing.T) {
	tokens, err := tokenize("max(a, b)")
	require.NoError(t, err)
	expected := []tokenType{tokIdent, tokLParen, tokIdent, tokComma, tokIdent, tokRParen, tokEOF}
	require.Len(t, tokens, len(expected))
	for i, exp := range expected {
		assert.Equal(t, exp, tokens[i].typ, "token %d", i)
	}
}

func TestTokenizeComplexExpression(t *testing.T) {
	input := "upstream_rate * instances * operation_cost / 1000 <= downstream_capacity * max(downstream_instances, 1)"
	tokens, err := tokenize(input)
	require.NoError(t, err)
	// upstream_rate * instances * operation_cost / 1000 <= downstream_capacity * max ( downstream_instances , 1 ) EOF
	// 16 tokens + EOF = 17
	assert.Len(t, tokens, 17)
	assert.Equal(t, tokEOF, tokens[len(tokens)-1].typ)
}

// --- Step 5.2: Parser — arithmetic ---

func TestParseNumber(t *testing.T) {
	ast, err := parseExpr("42")
	require.NoError(t, err)
	num, ok := ast.(*numberLiteral)
	require.True(t, ok)
	assert.Equal(t, 42.0, num.value)
}

func TestParseIdentifier(t *testing.T) {
	ast, err := parseExpr("x")
	require.NoError(t, err)
	id, ok := ast.(*identifier)
	require.True(t, ok)
	assert.Equal(t, "x", id.name)
}

func TestParseAddition(t *testing.T) {
	ast, err := parseExpr("a + b")
	require.NoError(t, err)
	bin, ok := ast.(*binaryOp)
	require.True(t, ok)
	assert.Equal(t, "+", bin.op)
	assert.Equal(t, "a", bin.left.(*identifier).name)
	assert.Equal(t, "b", bin.right.(*identifier).name)
}

func TestParseMultiplication(t *testing.T) {
	ast, err := parseExpr("a * b")
	require.NoError(t, err)
	bin, ok := ast.(*binaryOp)
	require.True(t, ok)
	assert.Equal(t, "*", bin.op)
}

func TestParsePrecedence(t *testing.T) {
	ast, err := parseExpr("a + b * c")
	require.NoError(t, err)
	bin, ok := ast.(*binaryOp)
	require.True(t, ok)
	assert.Equal(t, "+", bin.op)
	assert.Equal(t, "a", bin.left.(*identifier).name)
	mul := bin.right.(*binaryOp)
	assert.Equal(t, "*", mul.op)
}

func TestParseParentheses(t *testing.T) {
	ast, err := parseExpr("(a + b) * c")
	require.NoError(t, err)
	bin, ok := ast.(*binaryOp)
	require.True(t, ok)
	assert.Equal(t, "*", bin.op)
	add := bin.left.(*binaryOp)
	assert.Equal(t, "+", add.op)
}

func TestParseUnaryNegation(t *testing.T) {
	ast, err := parseExpr("-a")
	require.NoError(t, err)
	un, ok := ast.(*unaryOp)
	require.True(t, ok)
	assert.Equal(t, "-", un.op)
	assert.Equal(t, "a", un.operand.(*identifier).name)
}

func TestParseNestedParens(t *testing.T) {
	ast, err := parseExpr("((a))")
	require.NoError(t, err)
	id, ok := ast.(*identifier)
	require.True(t, ok)
	assert.Equal(t, "a", id.name)
}

// --- Step 5.3: Parser — comparisons and functions ---

func TestParseComparison(t *testing.T) {
	ast, err := parseExpr("a <= b")
	require.NoError(t, err)
	bin, ok := ast.(*binaryOp)
	require.True(t, ok)
	assert.Equal(t, "<=", bin.op)
}

func TestParseComparisonWithArithmetic(t *testing.T) {
	ast, err := parseExpr("a * b <= c + d")
	require.NoError(t, err)
	bin, ok := ast.(*binaryOp)
	require.True(t, ok)
	assert.Equal(t, "<=", bin.op)
	assert.Equal(t, "*", bin.left.(*binaryOp).op)
	assert.Equal(t, "+", bin.right.(*binaryOp).op)
}

func TestParseFunctionMax(t *testing.T) {
	ast, err := parseExpr("max(a, b)")
	require.NoError(t, err)
	fn, ok := ast.(*funcCall)
	require.True(t, ok)
	assert.Equal(t, "max", fn.name)
	assert.Len(t, fn.args, 2)
}

func TestParseFunctionMin(t *testing.T) {
	ast, err := parseExpr("min(a, b)")
	require.NoError(t, err)
	fn, ok := ast.(*funcCall)
	require.True(t, ok)
	assert.Equal(t, "min", fn.name)
	assert.Len(t, fn.args, 2)
}

func TestParseFunctionInExpression(t *testing.T) {
	ast, err := parseExpr("max(a, 1) * b")
	require.NoError(t, err)
	bin, ok := ast.(*binaryOp)
	require.True(t, ok)
	assert.Equal(t, "*", bin.op)
	_, ok = bin.left.(*funcCall)
	assert.True(t, ok)
}

func TestParseFullCapacityChainExpr(t *testing.T) {
	expr := "upstream_rate * instances * operation_cost / 1000 <= downstream_capacity * max(downstream_instances, 1)"
	ast, err := parseExpr(expr)
	require.NoError(t, err)
	bin, ok := ast.(*binaryOp)
	require.True(t, ok)
	assert.Equal(t, "<=", bin.op)
}

// --- Step 5.4: Evaluator — numeric expressions ---

func TestEvalNumber(t *testing.T) {
	result, err := EvaluateNumeric("42", nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 42.0, *result)
}

func TestEvalVariable(t *testing.T) {
	result, err := EvaluateNumeric("x", map[string]any{"x": 10})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 10.0, *result)
}

func TestEvalAddition(t *testing.T) {
	result, err := EvaluateNumeric("a + b", map[string]any{"a": 3, "b": 4})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 7.0, *result)
}

func TestEvalSubtraction(t *testing.T) {
	result, err := EvaluateNumeric("a - b", map[string]any{"a": 10, "b": 3})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 7.0, *result)
}

func TestEvalMultiplication(t *testing.T) {
	result, err := EvaluateNumeric("a * b", map[string]any{"a": 3, "b": 4})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 12.0, *result)
}

func TestEvalDivision(t *testing.T) {
	result, err := EvaluateNumeric("a / b", map[string]any{"a": 12, "b": 4})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 3.0, *result)
}

func TestEvalDivisionByZero(t *testing.T) {
	_, err := EvaluateNumeric("a / b", map[string]any{"a": 12, "b": 0})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "division by zero")
}

func TestEvalPrecedence(t *testing.T) {
	result, err := EvaluateNumeric("a + b * c", map[string]any{"a": 1, "b": 2, "c": 3})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 7.0, *result)
}

func TestEvalUnaryNegation(t *testing.T) {
	result, err := EvaluateNumeric("-a", map[string]any{"a": 5})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, -5.0, *result)
}

func TestEvalNestedArithmetic(t *testing.T) {
	result, err := EvaluateNumeric("(a + b) * c / d", map[string]any{"a": 2, "b": 3, "c": 4, "d": 2})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 10.0, *result)
}

func TestEvalNestedFunctionCalls(t *testing.T) {
	// max(min(3, 7), 5) = max(3, 5) = 5
	result, err := EvaluateNumeric("max(min(a, b), c)", map[string]any{"a": 3, "b": 7, "c": 5})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 5.0, *result)
}

// --- Step 5.5: Evaluator — null propagation ---

func TestEvalNullVariable(t *testing.T) {
	result, err := EvaluateNumeric("x", map[string]any{"x": nil})
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestEvalNullPlusNumber(t *testing.T) {
	result, err := EvaluateNumeric("a + b", map[string]any{"a": nil, "b": 5})
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestEvalNullTimesNumber(t *testing.T) {
	result, err := EvaluateNumeric("a * b", map[string]any{"a": nil, "b": 5})
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestEvalNullDivNumber(t *testing.T) {
	result, err := EvaluateNumeric("a / b", map[string]any{"a": nil, "b": 5})
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestEvalNumberDivNull(t *testing.T) {
	result, err := EvaluateNumeric("a / b", map[string]any{"a": 5, "b": nil})
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestEvalUndefinedVariable(t *testing.T) {
	result, err := EvaluateNumeric("x", map[string]any{})
	require.NoError(t, err)
	assert.Nil(t, result)
}

// --- Step 5.6: Evaluator — comparisons ---

func TestEvalComparisonTrue(t *testing.T) {
	result, err := EvaluateComparison("a <= b", map[string]any{"a": 5, "b": 10})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, *result)
}

func TestEvalComparisonFalse(t *testing.T) {
	result, err := EvaluateComparison("a <= b", map[string]any{"a": 15, "b": 10})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, *result)
}

func TestEvalComparisonNullLeft(t *testing.T) {
	result, err := EvaluateComparison("a <= b", map[string]any{"a": nil, "b": 10})
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestEvalComparisonNullRight(t *testing.T) {
	result, err := EvaluateComparison("a <= b", map[string]any{"a": 5, "b": nil})
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestEvalAllComparisonOps(t *testing.T) {
	vars := map[string]any{"a": 5, "b": 10}

	tests := []struct {
		expr     string
		expected bool
	}{
		{"a < b", true},
		{"a > b", false},
		{"a <= b", true},
		{"a >= b", false},
		{"b > a", true},
		{"b >= a", true},
	}
	for _, tc := range tests {
		result, err := EvaluateComparison(tc.expr, vars)
		require.NoError(t, err, tc.expr)
		require.NotNil(t, result, tc.expr)
		assert.Equal(t, tc.expected, *result, tc.expr)
	}
}

// --- Step 5.7: Evaluator — functions ---

func TestEvalMax(t *testing.T) {
	result, err := EvaluateNumeric("max(a, b)", map[string]any{"a": 3, "b": 7})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 7.0, *result)
}

func TestEvalMaxNullFirst(t *testing.T) {
	result, err := EvaluateNumeric("max(a, b)", map[string]any{"a": nil, "b": 7})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 7.0, *result)
}

func TestEvalMaxNullSecond(t *testing.T) {
	result, err := EvaluateNumeric("max(a, b)", map[string]any{"a": 3, "b": nil})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 3.0, *result)
}

func TestEvalMaxBothNull(t *testing.T) {
	result, err := EvaluateNumeric("max(a, b)", map[string]any{"a": nil, "b": nil})
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestEvalMin(t *testing.T) {
	result, err := EvaluateNumeric("min(a, b)", map[string]any{"a": 3, "b": 7})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 3.0, *result)
}

func TestEvalMinNullFirst(t *testing.T) {
	result, err := EvaluateNumeric("min(a, b)", map[string]any{"a": nil, "b": 7})
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestEvalMinNullSecond(t *testing.T) {
	result, err := EvaluateNumeric("min(a, b)", map[string]any{"a": 3, "b": nil})
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestEvalMinBothNull(t *testing.T) {
	result, err := EvaluateNumeric("min(a, b)", map[string]any{"a": nil, "b": nil})
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestEvalUnknownFunction(t *testing.T) {
	_, err := EvaluateNumeric("foo(a)", map[string]any{"a": 1})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown function")
}

// --- Step 5.8: Full spec expression integration ---

func TestEvalCapacityChainThroughputPass(t *testing.T) {
	expr := "upstream_rate * instances * operation_cost / 1000 <= downstream_capacity * max(downstream_instances, 1)"
	vars := map[string]any{
		"upstream_rate":        100,
		"instances":            2,
		"operation_cost":       10,
		"downstream_capacity":  200,
		"downstream_instances": 1,
	}
	// 100 * 2 * 10 / 1000 = 2 <= 200 * max(1, 1) = 200 → true
	result, err := EvaluateComparison(expr, vars)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, *result)
}

func TestEvalCapacityChainThroughputFail(t *testing.T) {
	expr := "upstream_rate * instances * operation_cost / 1000 <= downstream_capacity * max(downstream_instances, 1)"
	vars := map[string]any{
		"upstream_rate":        1000,
		"instances":            50,
		"operation_cost":       100,
		"downstream_capacity":  100,
		"downstream_instances": 1,
	}
	// 1000 * 50 * 100 / 1000 = 5000 <= 100 * 1 = 100 → false
	result, err := EvaluateComparison(expr, vars)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, *result)
}

func TestEvalPooledConnectionLimitPass(t *testing.T) {
	expr := "max_pool_size * instances <= downstream_capacity"
	vars := map[string]any{
		"max_pool_size":       10,
		"instances":           5,
		"downstream_capacity": 100,
	}
	// 10 * 5 = 50 <= 100 → true
	result, err := EvaluateComparison(expr, vars)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, *result)
}

func TestEvalPooledConnectionLimitFail(t *testing.T) {
	expr := "max_pool_size * instances <= downstream_capacity"
	vars := map[string]any{
		"max_pool_size":       20,
		"instances":           10,
		"downstream_capacity": 100,
	}
	// 20 * 10 = 200 <= 100 → false
	result, err := EvaluateComparison(expr, vars)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, *result)
}

func TestEvalPooledThroughputPass(t *testing.T) {
	expr := "max_pool_size * instances * 1000 / operation_cost <= downstream_max_ops"
	vars := map[string]any{
		"max_pool_size":      10,
		"instances":          5,
		"operation_cost":     50,
		"downstream_max_ops": 5000,
	}
	// 10 * 5 * 1000 / 50 = 1000 <= 5000 → true
	result, err := EvaluateComparison(expr, vars)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, *result)
}
