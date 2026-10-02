package strategy

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"crypto-scanner/internal/closedindicator"
	"crypto-scanner/internal/indicator"
	"crypto-scanner/internal/indicator/candle"
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/scannerindicator"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common"
	"cel.dev/cel-go/common/ast"
	"cel.dev/cel-go/common/operators"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/common/types/traits"
	"cel.dev/cel-go/parser"
)

const (
	maxExpressionLength = 2000
	// maxComparisons counts the comparisons of the source; a crossing counts
	// once.
	maxComparisons = 20
	// maxNodes bounds the expanded expression, which repeats the expression
	// of a percentile window once per candle.
	maxNodes = 20000
	// maxShift is the largest prev shift and maxWindow the largest
	// percentile window, both in closed candles.
	maxShift  = 200
	maxWindow = 500
	// shiftMarker joins a variable name and its shift in the hidden
	// variables holding earlier values, such as h_rsi__shift1.
	shiftMarker = "__shift"
	// symbolMarker joins a variable name and the coin of follows in the
	// hidden variables of another instrument, such as
	// h_rsi__of_BTCUSDT__shift1; the shift comes last.
	symbolMarker = "__of_"
	// divideFunction and percentileFunction are the internal functions that
	// division and percentile windows compile into.
	divideFunction     = "__divide__"
	percentileFunction = "__percentile__"
	// maxArguments bounds the arguments of min and max.
	maxArguments = 10
)

// Variable is one value expressions read, named after the indicator table
// title, such as d_rsi or h_macd_macdsignal, or a built-in candle field, such
// as h_close. Candle fields have no indicator.
type Variable struct {
	Name        string
	Label       string
	IndicatorID int64
	Target      closedindicator.Target
	Output      string
}

// Read is a variable read at the closed candle shift candles before the
// latest one, of the instrument named Symbol or, when Symbol is empty, of
// the evaluated instrument.
type Read struct {
	Variable Variable
	Symbol   string
	Shift    int
}

var symbolPattern = regexp.MustCompile(`^[A-Z0-9]{2,30}$`)

// candleVariables name the built-in candle field variables after their
// field, such as h_close or d_quote_volume.
var candleVariables = []struct{ name, field string }{
	{"open", "open"}, {"high", "high"}, {"low", "low"}, {"close", "close"},
	{"volume", "volume"}, {"quote_volume", "quote_asset_volume"}, {"trades", "trade_count"},
}

// CandleTarget is the closed-candle calculation of the candle fields of
// interval.
func CandleTarget(interval market.CandleInterval) closedindicator.Target {
	return closedindicator.Target{Interval: interval, Selection: indicator.Selection{Type: candle.Type, Parameters: indicator.Parameters{}}}
}

var invalidNameCharacters = regexp.MustCompile(`[^A-Za-z0-9_]`)

// Variables lists the built-in candle fields of every interval, then every
// output of the configured indicators in display order. Candle field names
// are reserved, and indicator creation rejects duplicate titles; should two
// outputs still share a name, the older indicator keeps it, so reordering
// never rebinds an expression. Names containing the reserved markers of
// hidden variables are left out.
func Variables(entries []scannerindicator.Entry) []Variable {
	var result []Variable
	taken := map[string]struct{}{}
	for _, interval := range market.CandleIntervals() {
		prefix := scannerindicator.IntervalPrefix(interval)
		for _, field := range candleVariables {
			name := prefix + "_" + field.name
			taken[name] = struct{}{}
			result = append(result, Variable{Name: name, Label: prefix + "-" + field.name, Target: CandleTarget(interval), Output: field.field})
		}
	}
	byAge := slices.Clone(entries)
	slices.SortFunc(byAge, func(left, right scannerindicator.Entry) int { return cmp.Compare(left.ID, right.ID) })
	type key struct {
		id     int64
		output string
	}
	names := map[key]string{}
	for _, entry := range byAge {
		base := invalidNameCharacters.ReplaceAllString(entry.Title, "_")
		for _, output := range entry.Outputs {
			name := base
			if len(entry.Outputs) > 1 {
				name += "_" + invalidNameCharacters.ReplaceAllString(output, "_")
			}
			if _, duplicate := taken[name]; duplicate || strings.Contains(name, shiftMarker) || strings.Contains(name, symbolMarker) {
				continue
			}
			taken[name] = struct{}{}
			names[key{entry.ID, output}] = name
		}
	}
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
	// reads maps the identifiers of the expression to what they read.
	reads map[string]Read
}

// IndicatorIDs lists the indicators the expression reads, ascending. Candle
// fields belong to no indicator.
func (expression *Expression) IndicatorIDs() []int64 {
	var ids []int64
	for _, read := range expression.reads {
		if id := read.Variable.IndicatorID; id != 0 && !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	return ids
}

// Symbols lists the other instruments the expression reads through of,
// ascending.
func (expression *Expression) Symbols() []string {
	var symbols []string
	for _, read := range expression.reads {
		if read.Symbol != "" && !slices.Contains(symbols, read.Symbol) {
			symbols = append(symbols, read.Symbol)
		}
	}
	slices.Sort(symbols)
	return symbols
}

// Reads lists every variable, instrument, and shift the expression reads, by
// name, symbol, and shift.
func (expression *Expression) Reads() []Read {
	result := make([]Read, 0, len(expression.reads))
	for _, read := range expression.reads {
		result = append(result, read)
	}
	slices.SortFunc(result, func(left, right Read) int {
		return cmp.Or(cmp.Compare(left.Variable.Name, right.Variable.Name), cmp.Compare(left.Symbol, right.Symbol), cmp.Compare(left.Shift, right.Shift))
	})
	return result
}

// Evaluate runs the expression over the values value reports. Missing values
// stay unbound, so CEL's commutative logic still decides branches that do not
// need them; known is false when the result depends on a missing value or a
// division by zero.
func (expression *Expression) Evaluate(value func(Read) (float64, bool)) (result bool, known bool) {
	activation := make(map[string]any, len(expression.reads))
	for name, read := range expression.reads {
		if number, ok := value(read); ok {
			activation[name] = number
		}
	}
	output, _, err := expression.program.Eval(activation)
	if err != nil {
		return false, false
	}
	matched, ok := output.Value().(bool)
	return matched, ok
}

// Compile checks source against the variables and prepares it for
// evaluation. Conditions are comparisons of arithmetic, abs, mod, min, and
// max over variables and numbers, combined with &&, ||, and !; prev,
// percentile, crosses_above, and crosses_below read earlier closed candles,
// and of reads another instrument.
//
// An invalid source fails with an *InvalidExpressionError listing every
// problem found.
func Compile(source string, variables []Variable) (*Expression, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return nil, invalidExpression("the expression is empty")
	}
	if len(source) > maxExpressionLength {
		return nil, invalidExpression(fmt.Sprintf("the expression is longer than %d characters", maxExpressionLength))
	}
	// Earlier values are read only through prev, percentile, and crossings,
	// and other instruments only through of.
	var reserved []string
	for _, marker := range []string{shiftMarker, symbolMarker} {
		if strings.Contains(source, marker) {
			reserved = append(reserved, fmt.Sprintf("names containing %s are reserved", marker))
		}
	}
	if len(reserved) > 0 {
		return nil, invalidExpression(reserved...)
	}
	byName := make(map[string]Variable, len(variables))
	for _, variable := range variables {
		byName[variable.Name] = variable
	}
	options := []cel.EnvOption{
		cel.ClearMacros(),
		cel.Macros(
			parser.NewGlobalMacro("prev", 1, prevMacro),
			parser.NewGlobalMacro("prev", 2, prevMacro),
			parser.NewGlobalMacro("percentile", 3, percentileMacro),
			parser.NewGlobalMacro("of", 2, ofMacro),
			crossesMacro("crosses_above", operators.LessEquals, operators.Greater),
			crossesMacro("crosses_below", operators.GreaterEquals, operators.Less),
		),
		cel.EnableMacroCallTracking(),
		cel.CrossTypeNumericComparisons(true),
		cel.ParserExpressionSizeLimit(maxExpressionLength),
		cel.Function(divideFunction, cel.Overload("divide_double_double", []*cel.Type{cel.DoubleType, cel.DoubleType}, cel.DoubleType, cel.BinaryBinding(divide))),
		cel.Function(percentileFunction, cel.Overload("percentile_double_list", []*cel.Type{cel.DoubleType, cel.ListType(cel.DoubleType)}, cel.DoubleType, cel.BinaryBinding(percentile))),
		cel.Function("abs", cel.Overload("abs_double", []*cel.Type{cel.DoubleType}, cel.DoubleType, cel.UnaryBinding(func(value ref.Val) ref.Val {
			return types.Double(math.Abs(float64(value.(types.Double))))
		}))),
		cel.Function("mod", cel.Overload("mod_double_double", []*cel.Type{cel.DoubleType, cel.DoubleType}, cel.DoubleType, cel.BinaryBinding(modulo))),
		cel.Function("min", extremeOverloads("min", math.Min)...),
		cel.Function("max", extremeOverloads("max", math.Max)...),
	}
	parseEnv, err := cel.NewEnv(options...)
	if err != nil {
		return nil, fmt.Errorf("create expression environment: %w", err)
	}
	parsed, issues := parseEnv.Parse(source)
	if issues.Err() != nil {
		return nil, invalidExpression(issueMessages(issues)...)
	}
	identifiers := map[string]Read{}
	rewrite(parsed.NativeRep().Expr(), ast.NewExprFactory(), byName, identifiers)
	// Every identifier is declared, so the walker names unknown ones.
	for name := range identifiers {
		options = append(options, cel.Variable(name, cel.DoubleType))
	}
	env, err := cel.NewEnv(options...)
	if err != nil {
		return nil, fmt.Errorf("create expression environment: %w", err)
	}
	checked, issues := env.Check(parsed)
	if issues.Err() != nil {
		return nil, invalidExpression(issueMessages(issues)...)
	}
	if !checked.OutputType().IsExactType(cel.BoolType) {
		return nil, invalidExpression("the expression must be a condition")
	}
	walker := expressionWalker{info: checked.NativeRep().SourceInfo(), identifiers: identifiers, reads: map[string]Read{}}
	if err := walker.condition(checked.NativeRep().Expr(), true); err != nil {
		return nil, invalidExpression(err.Error())
	}
	problems := walker.problems
	if walker.comparisons > maxComparisons {
		problems = append(problems, "the expression has too many comparisons")
	}
	if len(walker.problems) == 0 {
		if walker.comparisons == 0 {
			problems = append(problems, "the expression has no comparison")
		}
		// An expression that reads only other coins has the same result for
		// every evaluated coin, so all of them would match and alert at
		// once.
		evaluated := false
		for _, read := range walker.reads {
			evaluated = evaluated || read.Symbol == ""
		}
		if !evaluated {
			problems = append(problems, "the expression must also read the evaluated coin, not only coins read through of")
		}
	}
	if len(problems) > 0 {
		return nil, invalidExpression(problems...)
	}
	program, err := env.Program(checked)
	if err != nil {
		return nil, invalidExpression(err.Error())
	}
	return &Expression{program: program, reads: walker.reads}, nil
}

// InvalidExpressionError lists the problems of an expression that does not
// compile. It matches ErrInvalidArgument.
type InvalidExpressionError struct {
	Problems []string
}

func invalidExpression(problems ...string) *InvalidExpressionError {
	return &InvalidExpressionError{Problems: problems}
}

func (err *InvalidExpressionError) Error() string {
	return ErrInvalidArgument.Error() + ": " + strings.Join(err.Problems, "; ")
}

func (err *InvalidExpressionError) Is(target error) bool { return target == ErrInvalidArgument }

// issueMessages lists CEL issues by their line and column, without the
// drawing of the source with a caret.
func issueMessages(issues *cel.Issues) []string {
	var messages []string
	for _, issue := range issues.Errors() {
		message := fmt.Sprintf("%d:%d: %s", issue.Location.Line(), issue.Location.Column()+1, issue.Message)
		if !slices.Contains(messages, message) {
			messages = append(messages, message)
		}
	}
	return messages
}

// rewrite turns integer literals into doubles, since CEL has no mixed
// arithmetic, and division into divideFunction, which reports a zero divisor
// as an error. It records every identifier with what it reads, such as
// h_rsi__of_BTCUSDT__shift1; an identifier that names no variable reads a
// zero Variable.
func rewrite(expr ast.Expr, factory ast.ExprFactory, variables map[string]Variable, identifiers map[string]Read) {
	switch expr.Kind() {
	case ast.LiteralKind:
		switch literal := expr.AsLiteral().(type) {
		case types.Int:
			expr.SetKindCase(factory.NewLiteral(expr.ID(), types.Double(literal)))
		case types.Uint:
			expr.SetKindCase(factory.NewLiteral(expr.ID(), types.Double(literal)))
		}
	case ast.IdentKind:
		name, symbol, shift := splitIdentifier(expr.AsIdent())
		identifiers[expr.AsIdent()] = Read{Variable: variables[name], Symbol: symbol, Shift: shift}
	case ast.CallKind:
		call := expr.AsCall()
		for _, arg := range call.Args() {
			rewrite(arg, factory, variables, identifiers)
		}
		if call.IsMemberFunction() {
			rewrite(call.Target(), factory, variables, identifiers)
		} else if call.FunctionName() == operators.Divide {
			expr.SetKindCase(factory.NewCall(expr.ID(), divideFunction, call.Args()...))
		}
	case ast.ListKind:
		for _, element := range expr.AsList().Elements() {
			rewrite(element, factory, variables, identifiers)
		}
	}
}

// extremeOverloads declares name for 2 to maxArguments doubles, folding them
// with pick.
func extremeOverloads(name string, pick func(float64, float64) float64) []cel.FunctionOpt {
	var overloads []cel.FunctionOpt
	for count := 2; count <= maxArguments; count++ {
		arguments := make([]*cel.Type, count)
		for index := range arguments {
			arguments[index] = cel.DoubleType
		}
		overloads = append(overloads, cel.Overload(fmt.Sprintf("%s_double_%d", name, count), arguments, cel.DoubleType,
			cel.FunctionBinding(func(values ...ref.Val) ref.Val {
				result := float64(values[0].(types.Double))
				for _, value := range values[1:] {
					result = pick(result, float64(value.(types.Double)))
				}
				return types.Double(result)
			})))
	}
	return overloads
}

// modulo is the remainder of dividing left by right, with the sign of left;
// a zero divisor is an error, like division.
func modulo(left, right ref.Val) ref.Val {
	divisor := float64(right.(types.Double))
	if divisor == 0 {
		return types.NewErr("modulo by zero")
	}
	return types.Double(math.Mod(float64(left.(types.Double)), divisor))
}

func divide(left, right ref.Val) ref.Val {
	dividend, divisor := float64(left.(types.Double)), float64(right.(types.Double))
	quotient := dividend / divisor
	if divisor == 0 || math.IsNaN(quotient) || math.IsInf(quotient, 0) {
		return types.NewErr("division by zero")
	}
	return types.Double(quotient)
}

// percentile returns the nearest-rank percentile of values: the element at
// rank ceil(rank/100 × n) of the sorted values, counted from 1. The rank is
// a whole number, so the ceiling is exact integer arithmetic.
func percentile(rank, values ref.Val) ref.Val {
	lister, ok := values.(traits.Lister)
	if !ok {
		return types.NewErr("percentile needs a list")
	}
	size := int(lister.Size().(types.Int))
	numbers := make([]float64, 0, size)
	for index := range size {
		element := lister.Get(types.Int(index))
		number, ok := element.(types.Double)
		if !ok {
			// An unbound value or a failed division leaves the window
			// unknown.
			return element
		}
		numbers = append(numbers, float64(number))
	}
	if len(numbers) == 0 {
		return types.NewErr("percentile of no values")
	}
	slices.Sort(numbers)
	position := (int(rank.(types.Double))*len(numbers)+99)/100 - 1
	return types.Double(numbers[min(max(position, 0), len(numbers)-1)])
}

// splitIdentifier splits a hidden identifier such as
// h_rsi__of_BTCUSDT__shift3 into its variable name, symbol, and shift.
func splitIdentifier(identifier string) (string, string, int) {
	name, shift := splitShift(identifier)
	name, symbol, _ := strings.Cut(name, symbolMarker)
	return name, symbol, shift
}

// hiddenIdentifier joins name, which may name an instrument, and shift.
func hiddenIdentifier(name string, shift int) string {
	if shift == 0 {
		return name
	}
	return name + shiftMarker + strconv.Itoa(shift)
}

// splitShift splits a hidden identifier such as h_rsi__shift3 into the rest
// of the name and the shift.
func splitShift(identifier string) (string, int) {
	name, suffix, found := strings.Cut(identifier, shiftMarker)
	if !found {
		return identifier, 0
	}
	shift, err := strconv.Atoi(suffix)
	if err != nil || shift < 1 {
		return identifier, 0
	}
	return name, shift
}

// shifted returns a copy of expr reading every variable by more closed
// candles earlier.
func shifted(helper parser.ExprHelper, expr ast.Expr, by int) ast.Expr {
	result := helper.Copy(expr)
	renameIdentifiers(helper, result, func(identifier string) string {
		name, shift := splitShift(identifier)
		return hiddenIdentifier(name, shift+by)
	})
	return result
}

// renameIdentifiers replaces every identifier of expr with rename of it.
func renameIdentifiers(helper parser.ExprHelper, expr ast.Expr, rename func(string) string) {
	switch expr.Kind() {
	case ast.IdentKind:
		expr.SetKindCase(helper.NewIdent(rename(expr.AsIdent())))
	case ast.CallKind:
		call := expr.AsCall()
		if call.IsMemberFunction() {
			renameIdentifiers(helper, call.Target(), rename)
		}
		for _, arg := range call.Args() {
			renameIdentifiers(helper, arg, rename)
		}
	case ast.ListKind:
		for _, element := range expr.AsList().Elements() {
			renameIdentifiers(helper, element, rename)
		}
	}
}

// containsIdentifier reports whether an identifier of expr contains part.
func containsIdentifier(expr ast.Expr, part string) bool {
	found := false
	ast.PreOrderVisit(expr, ast.NewExprVisitor(func(node ast.Expr) {
		if node.Kind() == ast.IdentKind && strings.Contains(node.AsIdent(), part) {
			found = true
		}
	}))
	return found
}

// nodeCount counts the nodes of expr, including those inside map and
// struct literals.
func nodeCount(expr ast.Expr) int {
	count := 0
	ast.PreOrderVisit(expr, ast.NewExprVisitor(func(ast.Expr) { count++ }))
	return count
}

// containsCall reports whether expr calls function anywhere.
func containsCall(expr ast.Expr, function string) bool {
	found := false
	ast.PreOrderVisit(expr, ast.NewExprVisitor(func(node ast.Expr) {
		if node.Kind() == ast.CallKind && node.AsCall().FunctionName() == function {
			found = true
		}
	}))
	return found
}

// integerLiteral returns the value of a positive integer literal argument
// no greater than limit.
func integerLiteral(helper parser.ExprHelper, expr ast.Expr, what string, limit int) (int, *common.Error) {
	if expr.Kind() == ast.LiteralKind {
		if value, ok := expr.AsLiteral().(types.Int); ok && value >= 1 && int64(value) <= int64(limit) {
			return int(value), nil
		}
	}
	return 0, helper.NewError(expr.ID(), fmt.Sprintf("%s must be a whole number from 1 to %d", what, limit))
}

// prevMacro expands prev(x) and prev(x, n) into x read one or n closed
// candles earlier.
func prevMacro(helper parser.ExprHelper, _ ast.Expr, args []ast.Expr) (ast.Expr, *common.Error) {
	by := 1
	if len(args) == 2 {
		var err *common.Error
		if by, err = integerLiteral(helper, args[1], "the prev shift", maxShift); err != nil {
			return nil, err
		}
	}
	return shifted(helper, args[0], by), nil
}

// percentileMacro expands percentile(x, n, p) into the p-th percentile of x
// over the n closed candles before the latest one.
func percentileMacro(helper parser.ExprHelper, _ ast.Expr, args []ast.Expr) (ast.Expr, *common.Error) {
	window, err := integerLiteral(helper, args[1], "the percentile window", maxWindow)
	if err != nil {
		return nil, err
	}
	rank, err := integerLiteral(helper, args[2], "the percentile", 100)
	if err != nil {
		return nil, err
	}
	// The window repeats its operand, so nesting and size are checked
	// before copying anything.
	if containsCall(args[0], percentileFunction) {
		return nil, helper.NewError(args[0].ID(), "percentile cannot contain percentile")
	}
	if nodeCount(args[0])*window > maxNodes {
		return nil, helper.NewError(args[0].ID(), "the percentile window is too large")
	}
	elements := make([]ast.Expr, window)
	for index := range elements {
		elements[index] = shifted(helper, args[0], index+1)
	}
	return helper.NewCall(percentileFunction, helper.NewLiteral(types.Double(rank)), helper.NewList(elements...)), nil
}

// ofMacro expands of("BTCUSDT", x) into x read from that instrument. The
// symbol goes before the shift, so of and prev nest in either order.
func ofMacro(helper parser.ExprHelper, _ ast.Expr, args []ast.Expr) (ast.Expr, *common.Error) {
	var symbol string
	if args[0].Kind() == ast.LiteralKind {
		if value, ok := args[0].AsLiteral().(types.String); ok {
			symbol = market.NormalizeSymbol(string(value))
		}
	}
	if !symbolPattern.MatchString(symbol) {
		return nil, helper.NewError(args[0].ID(), `of takes a coin symbol in quotes, such as "BTCUSDT"`)
	}
	if containsIdentifier(args[1], symbolMarker) {
		return nil, helper.NewError(args[1].ID(), "of cannot contain of")
	}
	result := helper.Copy(args[1])
	renameIdentifiers(helper, result, func(identifier string) string {
		name, shift := splitShift(identifier)
		return hiddenIdentifier(name+symbolMarker+symbol, shift)
	})
	return result, nil
}

// crossesMacro expands name(a, b) into "a was before (a b) at the previous
// candle, and is after (a b) now", such as prev(a) <= prev(b) && a > b.
func crossesMacro(name, before, after string) cel.Macro {
	return parser.NewGlobalMacro(name, 2, func(helper parser.ExprHelper, _ ast.Expr, args []ast.Expr) (ast.Expr, *common.Error) {
		left, right := args[0], args[1]
		// Each operand appears twice, so nested crossings would grow
		// exponentially.
		if 2*(nodeCount(left)+nodeCount(right)) > maxNodes {
			return nil, helper.NewError(left.ID(), "the expression is too large")
		}
		return helper.NewCall(operators.LogicalAnd,
			helper.NewCall(before, shifted(helper, left, 1), shifted(helper, right, 1)),
			helper.NewCall(after, helper.Copy(left), helper.Copy(right)),
		), nil
	})
}

var (
	comparisonOperators = []string{operators.Greater, operators.GreaterEquals, operators.Less, operators.LessEquals}
	// arithmeticOperators are the operators and functions of values.
	arithmeticOperators = []string{operators.Add, operators.Subtract, operators.Multiply, divideFunction, "abs", "mod", "min", "max"}
)

// expressionWalker rejects everything but the strategy language and collects
// the variables it reads. It reports every problem in problems and keeps
// walking the other branches; only an expression too large stops it.
type expressionWalker struct {
	info        *ast.SourceInfo
	identifiers map[string]Read
	reads       map[string]Read
	problems    []string
	comparisons int
	nodes       int
	// variables counts the variables read by the current comparison.
	variables int
	// unknown is set when the current comparison reads an unknown name.
	unknown bool
}

// report records a problem once; percentile windows and crossings repeat
// their operands.
func (walker *expressionWalker) report(format string, args ...any) {
	problem := fmt.Sprintf(format, args...)
	if !slices.Contains(walker.problems, problem) {
		walker.problems = append(walker.problems, problem)
	}
}

func (walker *expressionWalker) visit() error {
	walker.nodes++
	if walker.nodes > maxNodes {
		return errors.New("the expression is too large")
	}
	return nil
}

// condition accepts &&, ||, and ! over comparisons. A comparison counts
// toward maxComparisons when count is set; a crossing counts once.
func (walker *expressionWalker) condition(expr ast.Expr, count bool) error {
	if err := walker.visit(); err != nil {
		return err
	}
	if expr.Kind() != ast.CallKind || expr.AsCall().IsMemberFunction() {
		walker.report("combine comparisons with &&, || and !")
		return nil
	}
	call := expr.AsCall()
	name := call.FunctionName()
	switch {
	case name == operators.LogicalAnd || name == operators.LogicalOr || name == operators.LogicalNot:
		if macro, ok := walker.info.GetMacroCall(expr.ID()); ok && count && macro.Kind() == ast.CallKind && strings.HasPrefix(macro.AsCall().FunctionName(), "crosses_") {
			walker.comparisons++
			count = false
		}
		for _, arg := range call.Args() {
			if err := walker.condition(arg, count); err != nil {
				return err
			}
		}
		return nil
	case slices.Contains(comparisonOperators, name):
		if count {
			walker.comparisons++
		}
		walker.variables = 0
		walker.unknown = false
		for _, arg := range call.Args() {
			if err := walker.value(arg, false); err != nil {
				return err
			}
		}
		// An unknown name reads no variable but is already reported.
		if walker.variables == 0 && !walker.unknown {
			walker.report("each comparison reads an indicator or a candle field")
		}
		return nil
	case slices.Contains(arithmeticOperators, name) || name == operators.Negate || name == percentileFunction:
		walker.report("compare values with >, >=, < or <=")
		return nil
	default:
		walker.report("%s is not supported", strings.Trim(name, "_"))
		return nil
	}
}

// value accepts arithmetic and abs, mod, min, and max over variables,
// numbers, and percentile windows, which cannot nest.
func (walker *expressionWalker) value(expr ast.Expr, inWindow bool) error {
	if err := walker.visit(); err != nil {
		return err
	}
	switch expr.Kind() {
	case ast.IdentKind:
		read := walker.identifiers[expr.AsIdent()]
		if read.Variable.Name == "" {
			name, _, _ := splitIdentifier(expr.AsIdent())
			walker.report("%s is not a configured indicator or candle field", name)
			walker.unknown = true
			return nil
		}
		walker.reads[expr.AsIdent()] = read
		walker.variables++
		return nil
	case ast.LiteralKind:
		if _, ok := expr.AsLiteral().(types.Double); ok {
			return nil
		}
	case ast.CallKind:
		call := expr.AsCall()
		if call.IsMemberFunction() {
			walker.report("functions are not supported")
			return nil
		}
		name := call.FunctionName()
		switch {
		case slices.Contains(arithmeticOperators, name) || name == operators.Negate:
			for _, arg := range call.Args() {
				if err := walker.value(arg, inWindow); err != nil {
					return err
				}
			}
			return nil
		case name == percentileFunction:
			if inWindow {
				walker.report("percentile cannot contain percentile")
				return nil
			}
			for _, element := range call.Args()[1].AsList().Elements() {
				if err := walker.value(element, true); err != nil {
					return err
				}
			}
			return nil
		case slices.Contains(comparisonOperators, name), name == operators.LogicalAnd, name == operators.LogicalOr, name == operators.LogicalNot:
			walker.report("comparisons cannot be compared or calculated with")
			return nil
		default:
			walker.report("%s is not supported", strings.Trim(name, "_"))
			return nil
		}
	}
	walker.report("calculate with indicators, candle fields, and numbers")
	return nil
}
