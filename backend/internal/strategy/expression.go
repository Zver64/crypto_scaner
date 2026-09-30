package strategy

import (
	"cmp"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"crypto-scanner/internal/closedindicator"
	"crypto-scanner/internal/scannerindicator"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common"
	"cel.dev/cel-go/common/ast"
	"cel.dev/cel-go/common/operators"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/parser"
)

const (
	maxExpressionLength = 2000
	// maxComparisons counts the comparisons of the source; a crossing counts
	// once.
	maxComparisons = 20
	// previousSuffix names the hidden variables holding the value at the
	// previous closed candle, which only crosses_above and crosses_below read.
	previousSuffix = "__prev"
)

// Variable is one indicator output that expressions read, named after the
// indicator table title, such as d_rsi or h_macd_macdsignal.
type Variable struct {
	Name        string
	Label       string
	IndicatorID int64
	Target      closedindicator.Target
	Output      string
}

var invalidNameCharacters = regexp.MustCompile(`[^A-Za-z0-9_]`)

// Variables lists every output of the configured indicators in display
// order. Indicator creation rejects duplicate titles; should two outputs
// still share a name, the older indicator keeps it, so reordering never
// rebinds an expression.
func Variables(entries []scannerindicator.Entry) []Variable {
	byAge := slices.Clone(entries)
	slices.SortFunc(byAge, func(left, right scannerindicator.Entry) int { return cmp.Compare(left.ID, right.ID) })
	type key struct {
		id     int64
		output string
	}
	names := map[key]string{}
	taken := map[string]struct{}{}
	for _, entry := range byAge {
		base := invalidNameCharacters.ReplaceAllString(entry.Title, "_")
		for _, output := range entry.Outputs {
			name := base
			if len(entry.Outputs) > 1 {
				name += "_" + invalidNameCharacters.ReplaceAllString(output, "_")
			}
			if _, duplicate := taken[name]; duplicate || strings.HasSuffix(name, previousSuffix) {
				continue
			}
			taken[name] = struct{}{}
			names[key{entry.ID, output}] = name
		}
	}
	var result []Variable
	for _, entry := range entries {
		for _, output := range entry.Outputs {
			name, ok := names[key{entry.ID, output}]
			if !ok {
				continue
			}
			label := entry.Title
			if len(entry.Outputs) > 1 {
				label += " " + output
			}
			result = append(result, Variable{Name: name, Label: label, IndicatorID: entry.ID, Target: entry.Target(), Output: output})
		}
	}
	return result
}

// Expression is a compiled strategy expression.
type Expression struct {
	program cel.Program
	// current and previous are the variables read at the latest and the
	// previous closed candle.
	current  []Variable
	previous []Variable
}

// IndicatorIDs lists the indicators the expression reads, ascending.
func (expression *Expression) IndicatorIDs() []int64 {
	var ids []int64
	for _, variable := range slices.Concat(expression.current, expression.previous) {
		if !slices.Contains(ids, variable.IndicatorID) {
			ids = append(ids, variable.IndicatorID)
		}
	}
	slices.Sort(ids)
	return ids
}

// Variables lists the variables read at the latest closed candle.
func (expression *Expression) Variables() []Variable { return slices.Clone(expression.current) }

// PreviousVariables lists the variables read at the previous closed candle.
func (expression *Expression) PreviousVariables() []Variable {
	return slices.Clone(expression.previous)
}

// Evaluate runs the expression over the known values. Missing variables stay
// unbound, so CEL's commutative logic still decides branches that do not need
// them; known is false when the result depends on a missing value.
func (expression *Expression) Evaluate(current, previous map[string]float64) (result bool, known bool) {
	activation := make(map[string]any, len(current)+len(previous))
	for name, value := range current {
		activation[name] = value
	}
	for name, value := range previous {
		activation[name+previousSuffix] = value
	}
	output, _, err := expression.program.Eval(activation)
	if err != nil {
		return false, false
	}
	matched, ok := output.Value().(bool)
	return matched, ok
}

// Compile checks source against the variables and prepares it for
// evaluation. Only comparisons of variables and numbers, &&, ||, !, and the
// crosses_above and crosses_below macros are accepted.
func Compile(source string, variables []Variable) (*Expression, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return nil, fmt.Errorf("%w: the expression is empty", ErrInvalidArgument)
	}
	if len(source) > maxExpressionLength {
		return nil, fmt.Errorf("%w: the expression is longer than %d characters", ErrInvalidArgument, maxExpressionLength)
	}
	// Previous values are read only through crosses_above and crosses_below.
	if strings.Contains(source, previousSuffix) {
		return nil, fmt.Errorf("%w: names ending in %s are reserved", ErrInvalidArgument, previousSuffix)
	}
	byName := make(map[string]Variable, len(variables))
	options := []cel.EnvOption{
		cel.ClearMacros(),
		cel.Macros(crossesMacro("crosses_above", operators.LessEquals, operators.Greater), crossesMacro("crosses_below", operators.GreaterEquals, operators.Less)),
		cel.CrossTypeNumericComparisons(true),
		cel.ParserExpressionSizeLimit(maxExpressionLength),
	}
	for _, variable := range variables {
		byName[variable.Name] = variable
		options = append(options, cel.Variable(variable.Name, cel.DoubleType), cel.Variable(variable.Name+previousSuffix, cel.DoubleType))
	}
	env, err := cel.NewEnv(options...)
	if err != nil {
		return nil, fmt.Errorf("create expression environment: %w", err)
	}
	checked, issues := env.Compile(source)
	if issues.Err() != nil {
		// Later lines only draw the source with a caret.
		message, _, _ := strings.Cut(issues.Err().Error(), "\n")
		message = strings.TrimPrefix(message, "ERROR: <input>:")
		return nil, fmt.Errorf("%w: %s", ErrInvalidArgument, message)
	}
	if !checked.OutputType().IsExactType(cel.BoolType) {
		return nil, fmt.Errorf("%w: the expression must be a condition", ErrInvalidArgument)
	}
	walker := expressionWalker{variables: byName}
	if err := walker.walk(checked.NativeRep().Expr()); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidArgument, err)
	}
	if walker.comparisons == 0 {
		return nil, fmt.Errorf("%w: the expression has no comparison", ErrInvalidArgument)
	}
	if walker.comparisons > maxComparisons {
		return nil, fmt.Errorf("%w: the expression has too many comparisons", ErrInvalidArgument)
	}
	program, err := env.Program(checked)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidArgument, err)
	}
	return &Expression{program: program, current: walker.current, previous: walker.previous}, nil
}

var comparisonOperators = []string{operators.Greater, operators.GreaterEquals, operators.Less, operators.LessEquals}

// expressionWalker rejects everything but the strategy language and collects
// the variables it reads.
type expressionWalker struct {
	variables   map[string]Variable
	current     []Variable
	previous    []Variable
	comparisons int
}

func (walker *expressionWalker) walk(expr ast.Expr) error {
	switch expr.Kind() {
	case ast.CallKind:
		call := expr.AsCall()
		name := call.FunctionName()
		switch {
		case call.IsMemberFunction():
			return errors.New("functions are not supported")
		case name == operators.LogicalAnd || name == operators.LogicalOr || name == operators.LogicalNot:
			for _, arg := range call.Args() {
				if err := walker.walk(arg); err != nil {
					return err
				}
			}
			return nil
		case slices.Contains(comparisonOperators, name):
			args := call.Args()
			if args[0].Kind() != ast.IdentKind {
				return errors.New("each comparison starts with an indicator")
			}
			// A crossing expands into a comparison of previous values,
			// which is not counted, and one of the latest values.
			if !strings.HasSuffix(args[0].AsIdent(), previousSuffix) {
				walker.comparisons++
			}
			for _, arg := range args {
				if err := walker.operand(arg); err != nil {
					return err
				}
			}
			return nil
		default:
			return fmt.Errorf("%s is not supported", strings.Trim(name, "_"))
		}
	}
	return errors.New("combine comparisons with &&, || and !")
}

// operand accepts a variable or a number.
func (walker *expressionWalker) operand(expr ast.Expr) error {
	switch expr.Kind() {
	case ast.IdentKind:
		name := expr.AsIdent()
		target := &walker.current
		if base, ok := strings.CutSuffix(name, previousSuffix); ok {
			name, target = base, &walker.previous
		}
		variable, ok := walker.variables[name]
		if !ok {
			return fmt.Errorf("%s is not a configured indicator", name)
		}
		if !slices.ContainsFunc(*target, func(known Variable) bool { return known.Name == name }) {
			*target = append(*target, variable)
		}
		return nil
	case ast.LiteralKind:
		switch expr.AsLiteral().Type() {
		case types.IntType, types.UintType, types.DoubleType:
			return nil
		}
	case ast.CallKind:
		if call := expr.AsCall(); call.FunctionName() == operators.Negate && len(call.Args()) == 1 && call.Args()[0].Kind() == ast.LiteralKind {
			return walker.operand(call.Args()[0])
		}
	}
	return errors.New("compare indicators with numbers or other indicators")
}

// crossesMacro expands name(a, b) into "a was before (a b) at the previous
// candle, and is after (a b) now", such as a__prev <= b__prev && a > b.
func crossesMacro(name, before, after string) cel.Macro {
	return parser.NewGlobalMacro(name, 2, func(helper parser.ExprHelper, _ ast.Expr, args []ast.Expr) (ast.Expr, *common.Error) {
		left, right := args[0], args[1]
		if left.Kind() != ast.IdentKind {
			return nil, helper.NewError(left.ID(), name+" needs an indicator as its first argument")
		}
		var previousRight ast.Expr
		switch {
		case right.Kind() == ast.IdentKind:
			previousRight = helper.NewIdent(right.AsIdent() + previousSuffix)
		case right.Kind() == ast.LiteralKind, right.Kind() == ast.CallKind && right.AsCall().FunctionName() == operators.Negate:
			previousRight = helper.Copy(right)
		default:
			return nil, helper.NewError(right.ID(), name+" needs an indicator or a number as its second argument")
		}
		return helper.NewCall(operators.LogicalAnd,
			helper.NewCall(before, helper.NewIdent(left.AsIdent()+previousSuffix), previousRight),
			helper.NewCall(after, helper.Copy(left), helper.Copy(right)),
		), nil
	})
}
