package check

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// Token types
type tokenType int

const (
	tokNumber tokenType = iota
	tokIdent
	tokPlus
	tokMinus
	tokStar
	tokSlash
	tokLTE
	tokGTE
	tokLT
	tokGT
	tokLParen
	tokRParen
	tokComma
	tokEOF
)

type token struct {
	typ tokenType
	val string
}

// Tokenize splits an expression string into tokens.
func tokenize(input string) ([]token, error) {
	var tokens []token
	i := 0
	for i < len(input) {
		ch := input[i]

		// Skip whitespace
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			i++
			continue
		}

		// Number
		if ch >= '0' && ch <= '9' || ch == '.' {
			start := i
			for i < len(input) && (input[i] >= '0' && input[i] <= '9' || input[i] == '.') {
				i++
			}
			tokens = append(tokens, token{tokNumber, input[start:i]})
			continue
		}

		// Identifier
		if ch == '_' || unicode.IsLetter(rune(ch)) {
			start := i
			for i < len(input) && (input[i] == '_' || unicode.IsLetter(rune(input[i])) || unicode.IsDigit(rune(input[i]))) {
				i++
			}
			tokens = append(tokens, token{tokIdent, input[start:i]})
			continue
		}

		// Two-character operators
		if i+1 < len(input) {
			two := input[i : i+2]
			if two == "<=" {
				tokens = append(tokens, token{tokLTE, "<="})
				i += 2
				continue
			}
			if two == ">=" {
				tokens = append(tokens, token{tokGTE, ">="})
				i += 2
				continue
			}
		}

		// Single-character operators
		switch ch {
		case '+':
			tokens = append(tokens, token{tokPlus, "+"})
		case '-':
			tokens = append(tokens, token{tokMinus, "-"})
		case '*':
			tokens = append(tokens, token{tokStar, "*"})
		case '/':
			tokens = append(tokens, token{tokSlash, "/"})
		case '<':
			tokens = append(tokens, token{tokLT, "<"})
		case '>':
			tokens = append(tokens, token{tokGT, ">"})
		case '(':
			tokens = append(tokens, token{tokLParen, "("})
		case ')':
			tokens = append(tokens, token{tokRParen, ")"})
		case ',':
			tokens = append(tokens, token{tokComma, ","})
		default:
			return nil, fmt.Errorf("unexpected character %q at position %d", ch, i)
		}
		i++
	}

	tokens = append(tokens, token{tokEOF, ""})
	return tokens, nil
}

// AST node types

type node interface {
	nodeType() string
}

type numberLiteral struct {
	value float64
}

func (n *numberLiteral) nodeType() string { return "number" }

type identifier struct {
	name string
}

func (n *identifier) nodeType() string { return "identifier" }

type binaryOp struct {
	op    string
	left  node
	right node
}

func (n *binaryOp) nodeType() string { return "binary" }

type unaryOp struct {
	op      string
	operand node
}

func (n *unaryOp) nodeType() string { return "unary" }

type funcCall struct {
	name string
	args []node
}

func (n *funcCall) nodeType() string { return "func" }

// Parser

type parser struct {
	tokens []token
	pos    int
}

func newParser(tokens []token) *parser {
	return &parser{tokens: tokens, pos: 0}
}

func (p *parser) peek() token {
	if p.pos >= len(p.tokens) {
		return token{tokEOF, ""}
	}
	return p.tokens[p.pos]
}

func (p *parser) advance() token {
	t := p.peek()
	p.pos++
	return t
}

func (p *parser) expect(typ tokenType) (token, error) {
	t := p.advance()
	if t.typ != typ {
		return t, fmt.Errorf("expected token type %d, got %d (%q)", typ, t.typ, t.val)
	}
	return t, nil
}

// parseExpression parses a comparison expression (top-level).
func (p *parser) parseExpression() (node, error) {
	left, err := p.parseArithmetic()
	if err != nil {
		return nil, err
	}

	t := p.peek()
	if t.typ == tokLTE || t.typ == tokGTE || t.typ == tokLT || t.typ == tokGT {
		p.advance()
		right, err := p.parseArithmetic()
		if err != nil {
			return nil, err
		}
		return &binaryOp{op: t.val, left: left, right: right}, nil
	}

	return left, nil
}

// parseArithmetic handles + and -
func (p *parser) parseArithmetic() (node, error) {
	left, err := p.parseTerm()
	if err != nil {
		return nil, err
	}

	for p.peek().typ == tokPlus || p.peek().typ == tokMinus {
		op := p.advance()
		right, err := p.parseTerm()
		if err != nil {
			return nil, err
		}
		left = &binaryOp{op: op.val, left: left, right: right}
	}

	return left, nil
}

// parseTerm handles * and /
func (p *parser) parseTerm() (node, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}

	for p.peek().typ == tokStar || p.peek().typ == tokSlash {
		op := p.advance()
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = &binaryOp{op: op.val, left: left, right: right}
	}

	return left, nil
}

// parseUnary handles unary minus
func (p *parser) parseUnary() (node, error) {
	if p.peek().typ == tokMinus {
		p.advance()
		operand, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		return &unaryOp{op: "-", operand: operand}, nil
	}
	return p.parsePrimary()
}

// parsePrimary handles numbers, identifiers, function calls, and parenthesized expressions
func (p *parser) parsePrimary() (node, error) {
	t := p.peek()

	switch t.typ {
	case tokNumber:
		p.advance()
		v, err := strconv.ParseFloat(t.val, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid number %q: %w", t.val, err)
		}
		return &numberLiteral{value: v}, nil

	case tokIdent:
		p.advance()
		// Check for function call
		if p.peek().typ == tokLParen {
			p.advance() // consume (
			var args []node
			if p.peek().typ != tokRParen {
				arg, err := p.parseArithmetic()
				if err != nil {
					return nil, err
				}
				args = append(args, arg)
				for p.peek().typ == tokComma {
					p.advance()
					arg, err := p.parseArithmetic()
					if err != nil {
						return nil, err
					}
					args = append(args, arg)
				}
			}
			if _, err := p.expect(tokRParen); err != nil {
				return nil, fmt.Errorf("expected ')' in function call: %w", err)
			}
			return &funcCall{name: t.val, args: args}, nil
		}
		return &identifier{name: t.val}, nil

	case tokLParen:
		p.advance()
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(tokRParen); err != nil {
			return nil, fmt.Errorf("expected ')': %w", err)
		}
		return expr, nil

	default:
		return nil, fmt.Errorf("unexpected token %q", t.val)
	}
}

// parseExpr parses an expression string into an AST.
func parseExpr(input string) (node, error) {
	tokens, err := tokenize(input)
	if err != nil {
		return nil, err
	}
	p := newParser(tokens)
	expr, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	if p.peek().typ != tokEOF {
		return nil, fmt.Errorf("unexpected token %q after expression", p.peek().val)
	}
	return expr, nil
}

// Evaluator

type evalResult struct {
	value *float64
}

func floatVal(v float64) *float64 {
	return &v
}

func evalNode(n node, vars map[string]any) (*float64, error) {
	switch n := n.(type) {
	case *numberLiteral:
		return floatVal(n.value), nil

	case *identifier:
		v, ok := vars[n.name]
		if !ok || v == nil {
			return nil, nil // null
		}
		f, err := toFloat64(v)
		if err != nil {
			return nil, fmt.Errorf("variable %q: %w", n.name, err)
		}
		return floatVal(f), nil

	case *unaryOp:
		operand, err := evalNode(n.operand, vars)
		if err != nil {
			return nil, err
		}
		if operand == nil {
			return nil, nil
		}
		return floatVal(-*operand), nil

	case *binaryOp:
		return evalBinaryOp(n, vars)

	case *funcCall:
		return evalFuncCall(n, vars)

	default:
		return nil, fmt.Errorf("unknown node type: %T", n)
	}
}

func evalBinaryOp(n *binaryOp, vars map[string]any) (*float64, error) {
	left, err := evalNode(n.left, vars)
	if err != nil {
		return nil, err
	}
	right, err := evalNode(n.right, vars)
	if err != nil {
		return nil, err
	}

	// Comparison operators
	switch n.op {
	case "<=", ">=", "<", ">":
		// Handled at the EvaluateComparison level; if we get here via EvaluateNumeric, error
		if left == nil || right == nil {
			return nil, nil // indeterminate
		}
		var result bool
		switch n.op {
		case "<=":
			result = *left <= *right
		case ">=":
			result = *left >= *right
		case "<":
			result = *left < *right
		case ">":
			result = *left > *right
		}
		if result {
			return floatVal(1), nil
		}
		return floatVal(0), nil
	}

	// Null propagation for arithmetic
	if left == nil || right == nil {
		return nil, nil
	}

	switch n.op {
	case "+":
		return floatVal(*left + *right), nil
	case "-":
		return floatVal(*left - *right), nil
	case "*":
		return floatVal(*left * *right), nil
	case "/":
		if *right == 0 {
			return nil, fmt.Errorf("division by zero")
		}
		return floatVal(*left / *right), nil
	default:
		return nil, fmt.Errorf("unknown operator %q", n.op)
	}
}

func evalFuncCall(n *funcCall, vars map[string]any) (*float64, error) {
	switch n.name {
	case "max":
		if len(n.args) != 2 {
			return nil, fmt.Errorf("max() requires exactly 2 arguments")
		}
		a, err := evalNode(n.args[0], vars)
		if err != nil {
			return nil, err
		}
		b, err := evalNode(n.args[1], vars)
		if err != nil {
			return nil, err
		}
		// max(null, x) = x, max(x, null) = x, max(null, null) = null
		if a == nil && b == nil {
			return nil, nil
		}
		if a == nil {
			return b, nil
		}
		if b == nil {
			return a, nil
		}
		if *a >= *b {
			return a, nil
		}
		return b, nil

	case "min":
		if len(n.args) != 2 {
			return nil, fmt.Errorf("min() requires exactly 2 arguments")
		}
		a, err := evalNode(n.args[0], vars)
		if err != nil {
			return nil, err
		}
		b, err := evalNode(n.args[1], vars)
		if err != nil {
			return nil, err
		}
		// min(null, x) = null, min(x, null) = null
		if a == nil || b == nil {
			return nil, nil
		}
		if *a <= *b {
			return a, nil
		}
		return b, nil

	default:
		return nil, fmt.Errorf("unknown function %q", n.name)
	}
}

func toFloat64(v any) (float64, error) {
	switch v := v.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case string:
		return 0, fmt.Errorf("cannot convert string %q to number", v)
	default:
		return 0, fmt.Errorf("cannot convert %T to number", v)
	}
}

// EvaluateNumeric evaluates an arithmetic expression. Returns nil for null/indeterminate.
func EvaluateNumeric(expr string, vars map[string]any) (*float64, error) {
	ast, err := parseExpr(expr)
	if err != nil {
		return nil, err
	}
	return evalNode(ast, vars)
}

// EvaluateComparison evaluates a comparison expression. Returns nil for indeterminate.
func EvaluateComparison(expr string, vars map[string]any) (*bool, error) {
	ast, err := parseExpr(expr)
	if err != nil {
		return nil, err
	}

	// Check if top-level is a comparison
	binOp, ok := ast.(*binaryOp)
	if !ok || !isComparisonOp(binOp.op) {
		return nil, fmt.Errorf("expression is not a comparison: %q", expr)
	}

	left, err := evalNode(binOp.left, vars)
	if err != nil {
		return nil, err
	}
	right, err := evalNode(binOp.right, vars)
	if err != nil {
		return nil, err
	}

	if left == nil || right == nil {
		return nil, nil // indeterminate
	}

	var result bool
	switch binOp.op {
	case "<=":
		result = *left <= *right
	case ">=":
		result = *left >= *right
	case "<":
		result = *left < *right
	case ">":
		result = *left > *right
	}
	return &result, nil
}

// FormatExpressionWithValues returns the expression with variable values substituted.
func FormatExpressionWithValues(expr string, vars map[string]any) string {
	result := expr
	for k, v := range vars {
		if v == nil {
			result = strings.ReplaceAll(result, k, "null")
		} else {
			f, err := toFloat64(v)
			if err == nil {
				result = strings.ReplaceAll(result, k, strconv.FormatFloat(f, 'f', -1, 64))
			}
		}
	}
	return result
}

func isComparisonOp(op string) bool {
	return op == "<=" || op == ">=" || op == "<" || op == ">"
}
