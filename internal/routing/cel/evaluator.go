package cel

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/cel-go/cel"

	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

var (
	// ErrEmptyExpression is returned when an empty expression is provided.
	ErrEmptyExpression = errors.New("empty CEL expression")

	// ErrCompilationFailed is returned when expression compilation fails.
	ErrCompilationFailed = errors.New("CEL expression compilation failed")

	// ErrEvaluationFailed is returned when expression evaluation fails.
	ErrEvaluationFailed = errors.New("CEL expression evaluation failed")

	// ErrNotBoolean is returned when expression does not return a boolean.
	ErrNotBoolean = errors.New("CEL expression must return a boolean value")
)

// CELEvaluator provides CEL expression compilation and evaluation for routing conditions.
type CELEvaluator interface {
	// Compile compiles and validates a CEL expression.
	Compile(expression string) (*CompiledExpression, error)

	// Evaluate evaluates a compiled expression against an alert and context.
	Evaluate(compiled *CompiledExpression, alert *routingv1.Alert, ctx *EvalContext) (bool, error)

	// EvaluateExpression compiles (or retrieves from cache) and evaluates an expression.
	EvaluateExpression(expression string, alert *routingv1.Alert, ctx *EvalContext) (bool, error)

	// Validate checks if an expression is valid without evaluating it.
	Validate(expression string) error
}

// CompiledExpression represents a compiled CEL expression ready for evaluation.
type CompiledExpression struct {
	Expression string
	Program    cel.Program
	AST        *cel.Ast
}

// Evaluator implements CELEvaluator with expression caching.
type Evaluator struct {
	cache *Cache
	env   *cel.Env
}

// EvaluatorOption configures an Evaluator.
type EvaluatorOption func(*Evaluator)

// WithCache sets a custom cache for the evaluator.
func WithCache(cache *Cache) EvaluatorOption {
	return func(e *Evaluator) {
		e.cache = cache
	}
}

// WithCacheCapacity sets the cache capacity.
func WithCacheCapacity(capacity int) EvaluatorOption {
	return func(e *Evaluator) {
		if e.cache != nil {
			// Cache already set, ignore
			return
		}
		cache, err := NewCache(capacity)
		if err == nil {
			e.cache = cache
		}
	}
}

// NewEvaluator creates a new CEL evaluator with optional configuration.
func NewEvaluator(opts ...EvaluatorOption) (*Evaluator, error) {
	env, err := NewStandardEnvironment()
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL environment: %w", err)
	}

	e := &Evaluator{
		env: env,
	}

	// Apply options
	for _, opt := range opts {
		opt(e)
	}

	// Create default cache if not provided
	if e.cache == nil {
		cache, err := NewCache(1000)
		if err != nil {
			return nil, fmt.Errorf("failed to create cache: %w", err)
		}
		e.cache = cache
	}

	return e, nil
}

// Compile compiles and validates a CEL expression.
func (e *Evaluator) Compile(expression string) (*CompiledExpression, error) {
	if expression == "" {
		return nil, ErrEmptyExpression
	}

	ast, issues := e.env.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("%w: %v", ErrCompilationFailed, issues.Err())
	}

	// Verify the expression returns a boolean
	outputType := ast.OutputType()
	if outputType != cel.BoolType {
		return nil, fmt.Errorf("%w: got %s", ErrNotBoolean, outputType)
	}

	prg, err := e.env.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCompilationFailed, err)
	}

	return &CompiledExpression{
		Expression: expression,
		Program:    prg,
		AST:        ast,
	}, nil
}

// Evaluate evaluates a compiled expression against an alert and context.
func (e *Evaluator) Evaluate(compiled *CompiledExpression, alert *routingv1.Alert, ctx *EvalContext) (bool, error) {
	if compiled == nil || compiled.Program == nil {
		return false, errors.New("nil compiled expression")
	}

	activation := BuildActivation(alert, ctx)

	result, _, err := compiled.Program.Eval(activation)
	if err != nil {
		return false, fmt.Errorf("%w: %v", ErrEvaluationFailed, err)
	}

	// Convert result to boolean
	boolVal, ok := result.Value().(bool)
	if !ok {
		return false, fmt.Errorf("%w: result type is %T", ErrNotBoolean, result.Value())
	}

	return boolVal, nil
}

// EvaluateExpression compiles (or retrieves from cache) and evaluates an expression.
func (e *Evaluator) EvaluateExpression(expression string, alert *routingv1.Alert, ctx *EvalContext) (bool, error) {
	if expression == "" {
		return false, ErrEmptyExpression
	}

	// Try to get from cache
	entry, err := e.cache.GetOrCompile(expression)
	if err != nil {
		return false, fmt.Errorf("%w: %v", ErrCompilationFailed, err)
	}

	activation := BuildActivation(alert, ctx)

	result, _, err := entry.Program.Eval(activation)
	if err != nil {
		return false, fmt.Errorf("%w: %v", ErrEvaluationFailed, err)
	}

	// Convert result to boolean
	boolVal, ok := result.Value().(bool)
	if !ok {
		return false, fmt.Errorf("%w: result type is %T", ErrNotBoolean, result.Value())
	}

	return boolVal, nil
}

// Validate checks if an expression is valid without evaluating it.
func (e *Evaluator) Validate(expression string) error {
	if expression == "" {
		return ErrEmptyExpression
	}

	ast, issues := e.env.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return fmt.Errorf("%w: %v", ErrCompilationFailed, issues.Err())
	}

	// Verify the expression returns a boolean
	if ast.OutputType() != cel.BoolType {
		return fmt.Errorf("%w: expression returns %s", ErrNotBoolean, ast.OutputType())
	}

	return nil
}

// Cache returns the expression cache.
func (e *Evaluator) Cache() *Cache {
	return e.cache
}

// Env returns the CEL environment.
func (e *Evaluator) Env() *cel.Env {
	return e.env
}

// EvaluateResult contains the result of an evaluation along with metadata.
type EvaluateResult struct {
	Matched    bool
	Expression string
	Duration   time.Duration
	CacheHit   bool
	Error      error
}

// EvaluateWithDetails evaluates an expression and returns detailed results.
func (e *Evaluator) EvaluateWithDetails(expression string, alert *routingv1.Alert, ctx *EvalContext) *EvaluateResult {
	start := time.Now()
	result := &EvaluateResult{
		Expression: expression,
	}

	if expression == "" {
		result.Error = ErrEmptyExpression
		result.Duration = time.Since(start)
		return result
	}

	// Check if in cache
	cached := e.cache.Get(expression)
	result.CacheHit = cached != nil

	matched, err := e.EvaluateExpression(expression, alert, ctx)
	result.Matched = matched
	result.Error = err
	result.Duration = time.Since(start)

	return result
}

// BatchEvaluate evaluates multiple expressions against the same alert.
func (e *Evaluator) BatchEvaluate(expressions []string, alert *routingv1.Alert, ctx *EvalContext) []*EvaluateResult {
	results := make([]*EvaluateResult, len(expressions))

	activation := BuildActivation(alert, ctx)

	for i, expr := range expressions {
		start := time.Now()
		result := &EvaluateResult{
			Expression: expr,
		}

		if expr == "" {
			result.Error = ErrEmptyExpression
			result.Duration = time.Since(start)
			results[i] = result
			continue
		}

		// Check if in cache
		cached := e.cache.Get(expr)
		result.CacheHit = cached != nil

		entry, err := e.cache.GetOrCompile(expr)
		if err != nil {
			result.Error = fmt.Errorf("%w: %v", ErrCompilationFailed, err)
			result.Duration = time.Since(start)
			results[i] = result
			continue
		}

		evalResult, _, err := entry.Program.Eval(activation)
		if err != nil {
			result.Error = fmt.Errorf("%w: %v", ErrEvaluationFailed, err)
			result.Duration = time.Since(start)
			results[i] = result
			continue
		}

		boolVal, ok := evalResult.Value().(bool)
		if !ok {
			result.Error = fmt.Errorf("%w: result type is %T", ErrNotBoolean, evalResult.Value())
		} else {
			result.Matched = boolVal
		}

		result.Duration = time.Since(start)
		results[i] = result
	}

	return results
}

// Ensure Evaluator implements CELEvaluator
var _ CELEvaluator = (*Evaluator)(nil)
