package billingexpr

import (
	"github.com/expr-lang/expr/ast"
	"github.com/expr-lang/expr/parser"
	"github.com/tidwall/sjson"
)

func IsFastServiceTier(tier string) bool {
	return tier == "fast" || tier == "priority"
}

// HasServiceTierPrice requires an explicit service_tier equality or membership
// condition. A tier label, comment or unrelated string is not a price rule.
// Inspect the unoptimized tree so an intentionally configured 1x rule survives.
func HasServiceTierPrice(expression, tier string) bool {
	_, body := ParseExprVersion(expression)
	tree, err := parser.Parse(body)
	if err != nil {
		return false
	}
	return ast.Find(tree.Node, func(n ast.Node) bool {
		binary, ok := n.(*ast.BinaryNode)
		if !ok {
			return false
		}
		if binary.Operator == "==" {
			return serviceTierParam(binary.Left) && stringNode(binary.Right, tier) ||
				serviceTierParam(binary.Right) && stringNode(binary.Left, tier)
		}
		if binary.Operator == "in" && serviceTierParam(binary.Left) {
			if values, ok := binary.Right.(*ast.ArrayNode); ok {
				for _, value := range values.Nodes {
					if stringNode(value, tier) {
						return true
					}
				}
			}
		}
		return false
	}) != nil
}

func serviceTierParam(n ast.Node) bool {
	call, ok := n.(*ast.CallNode)
	if !ok || len(call.Arguments) != 1 {
		return false
	}
	callee, ok := call.Callee.(*ast.IdentifierNode)
	return ok && callee.Value == "param" && stringNode(call.Arguments[0], "service_tier")
}

func stringNode(n ast.Node, value string) bool {
	str, ok := n.(*ast.StringNode)
	return ok && str.Value == value
}

// ServiceTierPriceInput resolves fast/priority aliases only for the billing
// probe. It never changes the JSON sent upstream. Explicit rules for both names
// retain their own prices; otherwise the configured equivalent is used.
func ServiceTierPriceInput(expression string, input RequestInput, tier string) (RequestInput, bool) {
	billingTier := tier
	if IsFastServiceTier(tier) && !HasServiceTierPrice(expression, tier) {
		billingTier = "fast"
		if tier == "fast" {
			billingTier = "priority"
		}
		if !HasServiceTierPrice(expression, billingTier) {
			return input, false
		}
	}
	body, err := sjson.SetBytes(input.Body, "service_tier", billingTier)
	if err != nil {
		return input, false
	}
	input.Body = body
	return input, true
}
