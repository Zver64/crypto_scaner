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
	"crypto-scanner/internal/market"
	"crypto-scanner/internal/platform/numeric"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common"
	"cel.dev/cel-go/common/ast"
	"cel.dev/cel-go/common/operators"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/common/types/traits"
	"cel.dev/cel-go/interpreter"
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
	// maxShift is the largest prev shift, written or calculated, and
	// maxWindow the largest percentile window, both in closed candles.
	maxShift  = 500
	maxWindow = 500
	// callMarker starts the hidden identifiers of calls, such as __call0.
	callMarker = "__call"
	// shiftMarker joins a hidden identifier and its shift in the hidden
	// identifiers holding earlier values, such as __call0__shift1.
	shiftMarker = "__shift"
	// symbolMarker joins a hidden identifier and the coin of follows in the
	// hidden identifiers of another instrument, such as
	// __call0__of_BTCUSDT__shift1; the shift comes last.
	symbolMarker = "__of_"
	// divideFunction, percentileFunction, and prevFunction are the internal
	// functions that division, percentile windows, and prev with a
	// calculated shift compile into.
	divideFunction     = "__divide__"
	percentileFunction = "__percentile__"
	prevFunction       = "__prev__"
	// prevMarker starts the hidden identifiers that replace the calls of
	// prevFunction in the program, such as __prev0.
	prevMarker = "__prev"
	// maxArguments bounds the arguments of Min and Max.
	maxArguments = 10
)

// Variable is one value expressions read: an output of an indicator or a
// candle field on an interval, named by its canonical call, such as
// rsi("1h"), bbands_upperband("1d", 20, 2.5), or close("1h"), or a position
// variable, such as pnl, which describes the open trade, has no target, and
// exists only in exit expressions.
type Variable struct {
	Name     string
	Target   closedindicator.Target
	Output   string
	Position bool
	// Count marks a count of candles, which a calculated prev shift may
	// read.
	Count bool
}

// Position variable names.
const (
	entryPriceVariable = "entry_price"
	pnlVariable        = "pnl"
	barsHeldVariable   = "bars_held"
)

// PositionVariables are the variables of the open trade that exit
// expressions read: the average price of its buys, its return at the close
// before fees, and the candles since its first buy filled.
func PositionVariables() []Variable {
	return []Variable{
		{Name: entryPriceVariable, Position: true},
		{Name: pnlVariable, Position: true},
		{Name: barsHeldVariable, Position: true, Count: true},
	}
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

// Expression is a compiled strategy expression.
type Expression struct {
	program cel.Program
	// reads maps the identifiers of the expression to what they read, and
	// shown those of them the source reads outside percentile windows and
	// the operands of calculated prev shifts.
	reads map[string]Read
	shown map[string]struct{}
	// prevs are the calculated prevs, by the identifiers that replace them
	// in the program.
	prevs map[string]calculatedPrev
}

// calculatedPrev is a prev with a calculated shift, evaluated on demand: the
// shift at the latest candle, then the operand that many candles earlier.
// operands are the identifiers of the operand.
type calculatedPrev struct {
	shift, operand cel.Program
	operands       []string
}

// ShiftError is a calculated prev shift that is not a whole number from 1 to
// maxShift at a candle: an error of the rule, unlike a value not known yet.
type ShiftError struct {
	Shift float64
}

func (err *ShiftError) Error() string {
	return fmt.Sprintf("the Prev shift is %s, not a whole number from 1 to %d", strconv.FormatFloat(err.Shift, 'g', -1, 64), maxShift)
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
// name, symbol, and shift, the operands of calculated prevs also at their
// deepest shift; position variables are left out, since they read no
// calculated value.
func (expression *Expression) Reads() []Read {
	result := make([]Read, 0, len(expression.reads))
	for _, read := range expression.reads {
		if !read.Variable.Position {
			result = append(result, read)
		}
	}
	for _, prev := range expression.prevs {
		for _, name := range prev.operands {
			if read := expression.reads[name]; !read.Variable.Position {
				read.Shift += maxShift
				result = append(result, read)
			}
		}
	}
	slices.SortFunc(result, func(left, right Read) int {
		return cmp.Or(cmp.Compare(left.Variable.Name, right.Variable.Name), cmp.Compare(left.Symbol, right.Symbol), cmp.Compare(left.Shift, right.Shift))
	})
	return result
}

// Evaluate runs the expression over the values value reports, position
// variables included. Missing values are unknown, so CEL's commutative logic
// still decides branches that do not need them; known is false when the
// result depends on a missing value or a division by zero. err is a
// *ShiftError when the result depends on an invalid calculated prev shift.
func (expression *Expression) Evaluate(value func(Read) (float64, bool)) (result, known bool, err error) {
	output, err := expression.eval(value)
	result, known = output.(bool)
	return result, known, err
}

// Price runs a price expression like Evaluate; known is also false when the
// price is not finite.
func (expression *Expression) Price(value func(Read) (float64, bool)) (price float64, known bool, err error) {
	output, err := expression.eval(value)
	price, known = output.(float64)
	return price, known && numeric.Finite(price), err
}

// eval returns the result, nil when it is not known.
func (expression *Expression) eval(value func(Read) (float64, bool)) (any, error) {
	output, _, _ := expression.program.Eval(activation{expression: expression, value: value})
	switch {
	case output == nil || types.IsUnknown(output):
		return nil, nil
	case types.IsError(output):
		var shift *ShiftError
		if errors.As(output.(*types.Err), &shift) {
			return nil, shift
		}
		return nil, nil
	}
	return output.Value(), nil
}

// activation resolves the identifiers of one evaluation on demand, offset
// candles earlier for the operand of a calculated prev. Values that value
// does not report are unknown.
type activation struct {
	expression *Expression
	value      func(Read) (float64, bool)
	offset     int
}

func (current activation) ResolveName(name string) (any, bool) {
	if prev, ok := current.expression.prevs[name]; ok {
		shift, failed := current.shift(prev)
		if failed != nil {
			return failed, true
		}
		output, _, _ := prev.operand.Eval(activation{expression: current.expression, value: current.value, offset: current.offset + shift})
		if output == nil {
			return types.NewUnknown(0, nil), true
		}
		return output, true
	}
	read, ok := current.expression.reads[name]
	if !ok {
		return nil, false
	}
	read.Shift += current.offset
	if number, ok := current.value(read); ok {
		return types.Double(number), true
	}
	return types.NewUnknown(0, nil), true
}

func (activation) Parent() interpreter.Activation { return nil }

// shift returns the shift of prev, or the unknown value or the *ShiftError
// instead.
func (current activation) shift(prev calculatedPrev) (int, ref.Val) {
	output, _, _ := prev.shift.Eval(current)
	shift, ok := output.(types.Double)
	if !ok {
		return 0, types.NewUnknown(0, nil)
	}
	if math.Trunc(float64(shift)) != float64(shift) || shift < 1 || shift > maxShift {
		return 0, types.WrapErr(&ShiftError{Shift: float64(shift)})
	}
	return int(shift), nil
}

// Values maps what the source reads outside percentile windows, position
// variables included, to the values value reports, named as the source
// writes them, such as rsi("1h"), Prev(rsi("1h"), 2), or
// Of("BTCUSDT", rsi("1h")). The operands of calculated prevs are named at the
// shift they read, such as Prev(rsi("1h"), 4). Missing values are left out.
func (expression *Expression) Values(value func(Read) (float64, bool)) map[string]float64 {
	values := make(map[string]float64, len(expression.shown))
	add := func(read Read) {
		if number, ok := value(read); ok {
			values[read.label()] = number
		}
	}
	for name := range expression.shown {
		add(expression.reads[name])
	}
	for _, prev := range expression.prevs {
		shift, failed := activation{expression: expression, value: value}.shift(prev)
		if failed != nil {
			continue
		}
		for _, name := range prev.operands {
			read := expression.reads[name]
			read.Shift += shift
			add(read)
		}
	}
	return values
}

// label names the read as the source writes it.
func (read Read) label() string {
	label := read.Variable.Name
	switch {
	case read.Shift == 1:
		label = "Prev(" + label + ")"
	case read.Shift > 1:
		label = fmt.Sprintf("Prev(%s, %d)", label, read.Shift)
	}
	if read.Symbol != "" {
		label = fmt.Sprintf("Of(%q, %s)", read.Symbol, label)
	}
	return label
}

// Compile checks source as a rule of kind and prepares it for evaluation.
// Conditions are comparisons of arithmetic, Abs, Mod, Min, and Max over
// indicator and candle field calls and numbers, combined with &&, ||, and !;
// Prev, Percentile, CrossesAbove, and CrossesBelow read earlier closed
// candles, and Of reads another instrument. An exit rule may also read the
// position variables. A price is such arithmetic that reads the evaluated
// instrument, such as close("1h") - 2 * atr("1h").
//
// An invalid source fails with an *InvalidExpressionError listing every
// problem found.
func Compile(source string, registry *indicator.Registry, kind RuleKind) (*Expression, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return nil, invalidExpression("the expression is empty")
	}
	if len(source) > maxExpressionLength {
		return nil, invalidExpression(fmt.Sprintf("the expression is longer than %d characters", maxExpressionLength))
	}
	// Earlier values are read only through Prev, Percentile, and crossings,
	// other instruments only through Of, and indicators only through calls.
	var reserved []string
	for _, marker := range []string{shiftMarker, symbolMarker, callMarker} {
		if strings.Contains(source, marker) {
			reserved = append(reserved, fmt.Sprintf("names containing %s are reserved", marker))
		}
	}
	if len(reserved) > 0 {
		return nil, invalidExpression(reserved...)
	}
	language := newLanguage(source, registry)
	options := language.options()
	parseEnv, err := cel.NewEnv(options...)
	if err != nil {
		return nil, fmt.Errorf("create expression environment: %w", err)
	}
	parsed, issues := parseEnv.Parse(source)
	if issues.Err() != nil {
		return nil, invalidExpression(issueMessages(issues)...)
	}
	// The macros expand percentile windows and crossings, so the expanded
	// expression is bounded before it is type-checked.
	if nodeCount(parsed.NativeRep().Expr()) > maxNodes {
		return nil, invalidExpression("the expression is too large")
	}
	rewriter := rewriter{language: language, factory: ast.NewExprFactory(), position: kind == ExitRule, identifiers: map[string]Read{}}
	rewriter.rewrite(parsed.NativeRep().Expr())
	if len(rewriter.problems) > 0 {
		return nil, invalidExpression(rewriter.problems...)
	}
	// Every identifier is declared, so the walker names unknown ones.
	for name := range rewriter.identifiers {
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
	walker := expressionWalker{info: checked.NativeRep().SourceInfo(), identifiers: rewriter.identifiers, reads: map[string]Read{}, shown: map[string]struct{}{}}
	price := kind == PriceRule
	if price {
		if !checked.OutputType().IsExactType(cel.DoubleType) {
			return nil, invalidExpression("the price must be calculated from indicators, candle fields, and numbers")
		}
		walker.value(checked.NativeRep().Expr(), "")
	} else {
		if !checked.OutputType().IsExactType(cel.BoolType) {
			return nil, invalidExpression("the expression must be a condition")
		}
		walker.condition(checked.NativeRep().Expr(), true)
	}
	problems := walker.problems
	if walker.comparisons > maxComparisons {
		problems = append(problems, "the expression has too many comparisons")
	}
	if len(walker.problems) == 0 {
		if !price && walker.comparisons == 0 {
			problems = append(problems, "the expression has no comparison")
		}
		// An expression that reads only other coins has the same result for
		// every evaluated coin, so all of them would match and alert at
		// once.
		evaluated := false
		for _, read := range walker.reads {
			evaluated = evaluated || read.Symbol == ""
		}
		if price && len(walker.reads) == 0 {
			problems = append(problems, "the price reads an indicator or a candle field")
		} else if !evaluated {
			problems = append(problems, "the expression must also read the evaluated coin, not only coins read through Of")
		}
	}
	if len(problems) > 0 {
		return nil, invalidExpression(problems...)
	}
	prevs, err := planPrevs(env, checked.NativeRep())
	if err != nil {
		return nil, invalidExpression(err.Error())
	}
	program, err := env.Program(checked)
	if err != nil {
		return nil, invalidExpression(err.Error())
	}
	return &Expression{program: program, reads: walker.reads, shown: walker.shown, prevs: prevs}, nil
}

// Canonical respells every call of source that resolves canonically, such
// as rsi("1h", 14) as rsi("1h"), and lists what the indicator calls
// calculate, in source order; the rest of source stays as written.
func Canonical(source string, registry *indicator.Registry) (string, []closedindicator.Target) {
	source = strings.TrimSpace(source)
	language := newLanguage(source, registry)
	env, err := cel.NewEnv(language.options()...)
	if err != nil || len(source) > maxExpressionLength {
		return source, nil
	}
	env.Parse(source)
	slices.SortFunc(language.calls, func(a, b call) int { return a.start - b.start })
	replacements := make([]replacement, 0, len(language.calls))
	var targets []closedindicator.Target
	for _, item := range language.calls {
		replacements = append(replacements, replacement{start: item.start, end: item.end, text: item.spelling})
		if !item.candle {
			targets = append(targets, item.target)
		}
	}
	return replace(source, replacements), targets
}

// language resolves the indicator and candle field calls of one source
// while it is parsed.
type language struct {
	registry *indicator.Registry
	source   []rune
	text     common.Source
	// calls are the calls resolved, each named in the expression by
	// callMarker and its index.
	calls []call
}

func newLanguage(source string, registry *indicator.Registry) *language {
	return &language{registry: registry, source: []rune(source), text: common.NewTextSource(source)}
}

// options declare the functions and macros of the language: a macro per
// indicator output and candle field resolves each call into an identifier
// before Prev, Of, Percentile, and the crossings expand around it. The bare
// type of an indicator with several outputs is an error naming them.
func (language *language) options() []cel.EnvOption {
	macros := []cel.Macro{
		parser.NewGlobalMacro("Prev", 1, prevMacro),
		parser.NewGlobalMacro("Prev", 2, prevMacro),
		parser.NewGlobalMacro("Percentile", 3, percentileMacro),
		parser.NewGlobalMacro("Of", 2, ofMacro),
		crossesMacro("CrossesAbove", operators.LessEquals, operators.Greater),
		crossesMacro("CrossesBelow", operators.GreaterEquals, operators.Less),
	}
	for _, field := range indicator.CandleFields {
		macros = append(macros, parser.NewGlobalVarArgMacro(field, language.callMacro(field, func(args []ast.Expr) (call, string) {
			return resolveCandleCall(field, args)
		})))
	}
	for _, descriptor := range language.registry.Descriptors() {
		if len(descriptor.Outputs) > 1 {
			problem := fmt.Sprintf("%s has several outputs; call one of %s", descriptor.Type, strings.Join(outputFunctions(descriptor), ", "))
			macros = append(macros, parser.NewGlobalVarArgMacro(string(descriptor.Type), func(parser.ExprHelper, ast.Expr, []ast.Expr) (ast.Expr, *common.Error) {
				return nil, &common.Error{Message: problem}
			}))
		}
		for _, output := range descriptor.Outputs {
			name := indicatorFunction(descriptor.Type, len(descriptor.Outputs), output.Name)
			macros = append(macros, parser.NewGlobalVarArgMacro(name, language.callMacro(name, func(args []ast.Expr) (call, string) {
				return resolveCall(language.registry, descriptor, output, args)
			})))
		}
	}
	return []cel.EnvOption{
		cel.ClearMacros(),
		cel.Macros(macros...),
		cel.EnableMacroCallTracking(),
		cel.CrossTypeNumericComparisons(true),
		cel.ParserExpressionSizeLimit(maxExpressionLength),
		cel.Function(divideFunction, cel.Overload("divide_double_double", []*cel.Type{cel.DoubleType, cel.DoubleType}, cel.DoubleType, cel.BinaryBinding(divide))),
		cel.Function(percentileFunction, cel.Overload("percentile_double_list", []*cel.Type{cel.DoubleType, cel.ListType(cel.DoubleType)}, cel.DoubleType, cel.BinaryBinding(percentile))),
		// Checked only: the program evaluates the shift and the operand on
		// demand (planPrevs).
		cel.Function(prevFunction, cel.Overload("prev_double_double", []*cel.Type{cel.DoubleType, cel.DoubleType}, cel.DoubleType)),
		cel.Function("Abs", cel.Overload("abs_double", []*cel.Type{cel.DoubleType}, cel.DoubleType, cel.UnaryBinding(func(value ref.Val) ref.Val {
			return types.Double(math.Abs(float64(value.(types.Double))))
		}))),
		cel.Function("Mod", cel.Overload("mod_double_double", []*cel.Type{cel.DoubleType, cel.DoubleType}, cel.DoubleType, cel.BinaryBinding(modulo))),
		cel.Function("Min", extremeOverloads("min", math.Min)...),
		cel.Function("Max", extremeOverloads("max", math.Max)...),
	}
}

// callMacro resolves a call of name through resolve into an identifier of
// the call, which rewrite turns into the variable it reads.
func (language *language) callMacro(name string, resolve func(args []ast.Expr) (call, string)) parser.MacroExpander {
	return func(helper parser.ExprHelper, _ ast.Expr, args []ast.Expr) (ast.Expr, *common.Error) {
		resolved, problem := resolve(args)
		if problem != "" {
			// Without a location, the error points at the call.
			return nil, &common.Error{Message: problem}
		}
		identifier := helper.NewIdent(callMarker + strconv.Itoa(len(language.calls)))
		// Expanded identifiers take the location of the call: its opening
		// parenthesis.
		open, found := language.text.LocationOffset(helper.OffsetLocation(identifier.ID()))
		if !found {
			return nil, &common.Error{Message: "cannot locate the call of " + name}
		}
		resolved.start, resolved.end = callSpan(language.source, int(open))
		language.calls = append(language.calls, resolved)
		return identifier, nil
	}
}

// planPrevs replaces every calculated prev of checked with a hidden
// identifier, which evaluation resolves on demand, and plans its shift and
// its operand as programs of their own.
func planPrevs(env *cel.Env, checked *ast.AST) (map[string]calculatedPrev, error) {
	var calls []ast.Expr
	ast.PreOrderVisit(checked.Expr(), ast.NewExprVisitor(func(node ast.Expr) {
		if node.Kind() == ast.CallKind && node.AsCall().FunctionName() == prevFunction {
			calls = append(calls, node)
		}
	}))
	plan := func(expr ast.Expr) (cel.Program, error) {
		return env.PlanProgram(ast.NewCheckedAST(ast.NewAST(expr, checked.SourceInfo()), checked.TypeMap(), checked.ReferenceMap()))
	}
	prevs := make(map[string]calculatedPrev, len(calls))
	for _, call := range calls {
		var prev calculatedPrev
		var err error
		args := call.AsCall().Args()
		if prev.shift, err = plan(args[0]); err != nil {
			return nil, err
		}
		if prev.operand, err = plan(args[1]); err != nil {
			return nil, err
		}
		ast.PreOrderVisit(args[1], ast.NewExprVisitor(func(node ast.Expr) {
			if node.Kind() == ast.IdentKind && !slices.Contains(prev.operands, node.AsIdent()) {
				prev.operands = append(prev.operands, node.AsIdent())
			}
		}))
		name := prevMarker + strconv.Itoa(len(prevs))
		prevs[name] = prev
		call.SetKindCase(ast.NewExprFactory().NewIdent(call.ID(), name))
		checked.SetReference(call.ID(), ast.NewIdentReference(name, nil))
	}
	return prevs, nil
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

// rewriter turns integer literals into doubles, since CEL has no mixed
// arithmetic, and division into divideFunction, which reports a zero divisor
// as an error. It turns the identifiers of calls into identifiers of
// variables, and records every identifier with what it reads, such as
// __call0__of_BTCUSDT__shift1; an identifier that names no variable reads a
// zero Variable. It reports calls of unknown functions and outputs, and
// fields, which the language lacks.
type rewriter struct {
	language *language
	factory  ast.ExprFactory
	// position allows the position variables.
	position    bool
	identifiers map[string]Read
	problems    []string
}

// functions are the functions of the language that remain calls after the
// macros expand.
var functions = []string{"Abs", "Mod", "Min", "Max", percentileFunction, prevFunction}

func (rewriter *rewriter) rewrite(expr ast.Expr) {
	switch expr.Kind() {
	case ast.LiteralKind:
		switch literal := expr.AsLiteral().(type) {
		case types.Int:
			expr.SetKindCase(rewriter.factory.NewLiteral(expr.ID(), types.Double(literal)))
		case types.Uint:
			expr.SetKindCase(rewriter.factory.NewLiteral(expr.ID(), types.Double(literal)))
		}
	case ast.IdentKind:
		rewriter.identifier(expr, expr.AsIdent())
	case ast.SelectKind:
		rewriter.report(`unknown field %s; an indicator output is part of the function name, such as bbands_upperband("1h")`, expr.AsSelect().FieldName())
	case ast.CallKind:
		call := expr.AsCall()
		for _, arg := range call.Args() {
			rewriter.rewrite(arg)
		}
		switch {
		case call.IsMemberFunction():
			rewriter.rewrite(call.Target())
		case call.FunctionName() == operators.Divide:
			expr.SetKindCase(rewriter.factory.NewCall(expr.ID(), divideFunction, call.Args()...))
		case !slices.Contains(functions, call.FunctionName()):
			if _, operator := operators.FindReverse(call.FunctionName()); !operator {
				rewriter.unknownFunction(call.FunctionName())
			}
		}
	case ast.ListKind:
		for _, element := range expr.AsList().Elements() {
			rewriter.rewrite(element)
		}
	}
}

// identifier replaces expr, which reads the hidden or written identifier,
// with the identifier of the variable it reads.
func (rewriter *rewriter) identifier(expr ast.Expr, identifier string) {
	name, symbol, shift := splitIdentifier(identifier)
	index, isCall := strings.CutPrefix(name, callMarker)
	if !isCall {
		read := Read{Symbol: symbol, Shift: shift}
		if position := slices.IndexFunc(PositionVariables(), func(variable Variable) bool { return variable.Name == name }); position >= 0 && rewriter.position {
			read.Variable = PositionVariables()[position]
		}
		rewriter.identifiers[identifier] = read
		return
	}
	number, _ := strconv.Atoi(index)
	variable := rewriter.language.calls[number].variable()
	if symbol != "" {
		name += symbolMarker + symbol
	}
	name = hiddenIdentifier(name, shift)
	rewriter.identifiers[name] = Read{Variable: variable, Symbol: symbol, Shift: shift}
	expr.SetKindCase(rewriter.factory.NewIdent(expr.ID(), name))
}

// unknownFunction reports the call of a function the language lacks,
// listing the outputs of an indicator whose output is unknown and pointing
// old lowercase names to the capitalized functions.
func (rewriter *rewriter) unknownFunction(name string) {
	for _, descriptor := range rewriter.language.registry.Descriptors() {
		if output, found := strings.CutPrefix(name, string(descriptor.Type)+"_"); found && len(descriptor.Outputs) > 1 {
			rewriter.report("%s has no output %s; call one of %s", descriptor.Type, output, strings.Join(outputFunctions(descriptor), ", "))
			return
		}
	}
	for _, function := range []string{"Prev", "Percentile", "Of", "CrossesAbove", "CrossesBelow", "Abs", "Mod", "Min", "Max"} {
		if strings.EqualFold(strings.ReplaceAll(name, "_", ""), function) {
			rewriter.report("unknown function %s; did you mean %s?", name, function)
			return
		}
	}
	rewriter.report("unknown function or indicator %s", name)
}

// report records a problem once; percentile windows and crossings repeat
// their operands.
func (rewriter *rewriter) report(format string, args ...any) {
	problem := fmt.Sprintf(format, args...)
	if !slices.Contains(rewriter.problems, problem) {
		rewriter.problems = append(rewriter.problems, problem)
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
	if divisor == 0 || !numeric.Finite(quotient) {
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
// __call0__of_BTCUSDT__shift3 into its base identifier, symbol, and shift.
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

// splitShift splits a hidden identifier such as __call0__shift3 into the rest
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
	case ast.SelectKind:
		renameIdentifiers(helper, expr.AsSelect().Operand(), rename)
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

// prevMacro expands Prev(x) and Prev(x, n) into x read one or n closed
// candles earlier. A shift that is not a number is calculated at the latest
// candle.
func prevMacro(helper parser.ExprHelper, _ ast.Expr, args []ast.Expr) (ast.Expr, *common.Error) {
	by := 1
	if len(args) == 2 {
		if args[1].Kind() != ast.LiteralKind {
			return calculatedPrevMacro(helper, args[0], args[1])
		}
		var err *common.Error
		if by, err = integerLiteral(helper, args[1], "the Prev shift", maxShift); err != nil {
			return nil, err
		}
	}
	return shifted(helper, args[0], by), nil
}

// calculatedPrevMacro expands Prev(x, n) with a calculated shift n into
// prevFunction over n and x, which the walker checks and planPrevs plans.
// The shift reads no earlier candle or other coin of its own; a Prev or Of
// around the whole call still shifts and moves both.
func calculatedPrevMacro(helper parser.ExprHelper, operand, shift ast.Expr) (ast.Expr, *common.Error) {
	if containsIdentifier(shift, shiftMarker) || containsIdentifier(shift, symbolMarker) {
		return nil, helper.NewError(shift.ID(), shiftProblem)
	}
	return helper.NewCall(prevFunction, shift, operand), nil
}

// shiftProblem rejects a calculated Prev shift that is not whole numbers
// and counts of candles added and subtracted, so it is a whole number by
// construction.
const shiftProblem = `a calculated Prev shift may only add and subtract whole numbers and candle counts, such as bars_held or pivot_low_bars("1h")`

// percentileMacro expands Percentile(x, n, p) into the p-th percentile of x
// over the n closed candles before the latest one.
func percentileMacro(helper parser.ExprHelper, _ ast.Expr, args []ast.Expr) (ast.Expr, *common.Error) {
	window, err := integerLiteral(helper, args[1], "the Percentile window", maxWindow)
	if err != nil {
		return nil, err
	}
	rank, err := integerLiteral(helper, args[2], "the Percentile rank", 100)
	if err != nil {
		return nil, err
	}
	// The window repeats its operand, so nesting and size are checked
	// before copying anything.
	if containsCall(args[0], percentileFunction) {
		return nil, helper.NewError(args[0].ID(), "Percentile cannot contain Percentile")
	}
	if containsCall(args[0], prevFunction) {
		return nil, helper.NewError(args[0].ID(), "Percentile cannot contain Prev with a calculated shift")
	}
	if nodeCount(args[0])*window > maxNodes {
		return nil, helper.NewError(args[0].ID(), "the Percentile window is too large")
	}
	elements := make([]ast.Expr, window)
	for index := range elements {
		elements[index] = shifted(helper, args[0], index+1)
	}
	return helper.NewCall(percentileFunction, helper.NewLiteral(types.Double(rank)), helper.NewList(elements...)), nil
}

// ofMacro expands Of("BTCUSDT", x) into x read from that instrument. The
// symbol goes before the shift, so Of and Prev nest in either order.
func ofMacro(helper parser.ExprHelper, _ ast.Expr, args []ast.Expr) (ast.Expr, *common.Error) {
	var symbol string
	if args[0].Kind() == ast.LiteralKind {
		if value, ok := args[0].AsLiteral().(types.String); ok {
			symbol = market.NormalizeSymbol(string(value))
		}
	}
	if !symbolPattern.MatchString(symbol) {
		return nil, helper.NewError(args[0].ID(), `Of takes a coin symbol in quotes, such as "BTCUSDT"`)
	}
	if containsIdentifier(args[1], symbolMarker) {
		return nil, helper.NewError(args[1].ID(), "Of cannot contain Of")
	}
	result := helper.Copy(args[1])
	renameIdentifiers(helper, result, func(identifier string) string {
		name, shift := splitShift(identifier)
		return hiddenIdentifier(name+symbolMarker+symbol, shift)
	})
	return result, nil
}

// crossesMacro expands name(a, b) into "a was before (a b) at the previous
// candle, and is after (a b) now", such as Prev(a) <= Prev(b) && a > b.
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
	arithmeticOperators = []string{operators.Add, operators.Subtract, operators.Multiply, divideFunction, "Abs", "Mod", "Min", "Max"}
)

// expressionWalker rejects everything but the strategy language and collects
// the variables it reads. It reports every problem in problems and keeps
// walking the other branches.
type expressionWalker struct {
	info        *ast.SourceInfo
	identifiers map[string]Read
	reads       map[string]Read
	// shown are the identifiers read outside percentile windows and the
	// operands of calculated prevs.
	shown    map[string]struct{}
	problems []string
	// shift is set while walking a calculated prev shift.
	shift       bool
	comparisons int
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

// condition accepts &&, ||, and ! over comparisons. A comparison counts
// toward maxComparisons when count is set; a crossing counts once.
func (walker *expressionWalker) condition(expr ast.Expr, count bool) {
	if expr.Kind() != ast.CallKind || expr.AsCall().IsMemberFunction() {
		walker.report("combine comparisons with &&, || and !")
		return
	}
	call := expr.AsCall()
	name := call.FunctionName()
	switch {
	case name == operators.LogicalAnd || name == operators.LogicalOr || name == operators.LogicalNot:
		if macro, ok := walker.info.GetMacroCall(expr.ID()); ok && count && macro.Kind() == ast.CallKind && strings.HasPrefix(macro.AsCall().FunctionName(), "Crosses") {
			walker.comparisons++
			count = false
		}
		for _, arg := range call.Args() {
			walker.condition(arg, count)
		}
	case slices.Contains(comparisonOperators, name):
		if count {
			walker.comparisons++
		}
		walker.variables = 0
		walker.unknown = false
		for _, arg := range call.Args() {
			walker.value(arg, "")
		}
		// An unknown name reads no variable but is already reported.
		if walker.variables == 0 && !walker.unknown {
			walker.report("each comparison reads an indicator or a candle field")
		}
	case slices.Contains(arithmeticOperators, name) || name == operators.Negate || name == percentileFunction || name == prevFunction:
		walker.report("compare values with >, >=, < or <=")
	default:
		walker.report("%s is not supported", strings.Trim(name, "_"))
	}
}

// value accepts arithmetic and Abs, Mod, Min, and Max over variables,
// numbers, percentile windows, and Prev with a calculated shift. window
// names the percentile or calculated prev whose operand expr is part of,
// empty outside; they cannot nest. A calculated prev shift is walked outside
// its window.
func (walker *expressionWalker) value(expr ast.Expr, window string) {
	switch expr.Kind() {
	case ast.IdentKind:
		read := walker.identifiers[expr.AsIdent()]
		if read.Variable.Name == "" {
			name, _, _ := splitIdentifier(expr.AsIdent())
			if slices.ContainsFunc(PositionVariables(), func(variable Variable) bool { return variable.Name == name }) {
				walker.report("%s is available only in the exit rule", name)
				walker.unknown = true
				return
			}
			walker.report(`unknown name %s; read indicators and candle fields through calls, such as close("1h")`, name)
			walker.unknown = true
			return
		}
		// Position variables exist only at the latest candle, so not in the
		// operand of a calculated prev either, which is read earlier.
		if read.Variable.Position && (read.Shift > 0 || read.Symbol != "" || window != "") {
			walker.report("%s cannot be read through Prev, Percentile, crossings, or Of", read.Variable.Name)
		}
		if walker.shift && !read.Variable.Count {
			walker.report("%s does not count candles, so it cannot be a Prev shift", read.Variable.Name)
		}
		walker.reads[expr.AsIdent()] = read
		if window == "" {
			walker.shown[expr.AsIdent()] = struct{}{}
		}
		walker.variables++
		return
	case ast.LiteralKind:
		if number, ok := expr.AsLiteral().(types.Double); ok {
			if walker.shift && math.Trunc(float64(number)) != float64(number) {
				walker.report(shiftProblem)
			}
			return
		}
	case ast.CallKind:
		call := expr.AsCall()
		if call.IsMemberFunction() {
			walker.report("functions are not supported")
			return
		}
		name := call.FunctionName()
		if walker.shift && name != operators.Add && name != operators.Subtract && name != operators.Negate {
			walker.report(shiftProblem)
			return
		}
		switch {
		case slices.Contains(arithmeticOperators, name) || name == operators.Negate:
			for _, arg := range call.Args() {
				walker.value(arg, window)
			}
			return
		case name == percentileFunction:
			if window != "" {
				walker.report("%s cannot contain Percentile", window)
				return
			}
			for _, element := range call.Args()[1].AsList().Elements() {
				walker.value(element, "Percentile")
			}
			return
		case name == prevFunction:
			if window != "" {
				walker.report("%s cannot contain Prev with a calculated shift", window)
				return
			}
			walker.shift = true
			walker.value(call.Args()[0], "")
			walker.shift = false
			walker.value(call.Args()[1], "Prev with a calculated shift")
			return
		case slices.Contains(comparisonOperators, name), name == operators.LogicalAnd, name == operators.LogicalOr, name == operators.LogicalNot:
			walker.report("comparisons cannot be compared or calculated with")
			return
		default:
			walker.report("%s is not supported", strings.Trim(name, "_"))
			return
		}
	}
	walker.report("calculate with indicators, candle fields, and numbers")
}
