package cel

import (
	"regexp"
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
)

// RegisterCustomFunctions returns a CEL environment option that registers custom functions.
func RegisterCustomFunctions() cel.EnvOption {
	return cel.Lib(&customFunctions{})
}

type customFunctions struct{}

// LibraryName implements cel.Library.
func (c *customFunctions) LibraryName() string {
	return "alerting.routing.functions"
}

// CompileOptions implements cel.Library.
func (c *customFunctions) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		// contains(list, item) - check if a list contains an item
		cel.Function("contains",
			cel.Overload("contains_list_string",
				[]*cel.Type{cel.ListType(cel.StringType), cel.StringType},
				cel.BoolType,
				cel.BinaryBinding(containsListString),
			),
			cel.Overload("contains_map_key",
				[]*cel.Type{cel.MapType(cel.StringType, cel.StringType), cel.StringType},
				cel.BoolType,
				cel.BinaryBinding(containsMapKey),
			),
		),

		// regexMatch(string, pattern) - regex match
		cel.Function("regexMatch",
			cel.Overload("regexmatch_string_string",
				[]*cel.Type{cel.StringType, cel.StringType},
				cel.BoolType,
				cel.BinaryBinding(matchesRegex),
			),
		),

		// startsWith(string, prefix) - check if string starts with prefix
		cel.Function("startsWith",
			cel.Overload("startswith_string_string",
				[]*cel.Type{cel.StringType, cel.StringType},
				cel.BoolType,
				cel.BinaryBinding(startsWithString),
			),
		),

		// endsWith(string, suffix) - check if string ends with suffix
		cel.Function("endsWith",
			cel.Overload("endswith_string_string",
				[]*cel.Type{cel.StringType, cel.StringType},
				cel.BoolType,
				cel.BinaryBinding(endsWithString),
			),
		),

		// hasLabel(labels, key) - check if labels map has a key
		cel.Function("hasLabel",
			cel.Overload("haslabel_map_string",
				[]*cel.Type{cel.MapType(cel.StringType, cel.StringType), cel.StringType},
				cel.BoolType,
				cel.BinaryBinding(containsMapKey),
			),
		),

		// getLabel(labels, key, default) - get label value or default
		cel.Function("getLabel",
			cel.Overload("getlabel_map_string_string",
				[]*cel.Type{cel.MapType(cel.StringType, cel.StringType), cel.StringType, cel.StringType},
				cel.StringType,
				cel.FunctionBinding(getLabelWithDefault),
			),
		),

		// labelEquals(labels, key, value) - check if label equals value
		cel.Function("labelEquals",
			cel.Overload("labelequals_map_string_string",
				[]*cel.Type{cel.MapType(cel.StringType, cel.StringType), cel.StringType, cel.StringType},
				cel.BoolType,
				cel.FunctionBinding(labelEquals),
			),
		),

		// labelIn(labels, key, values) - check if label value is in list
		cel.Function("labelIn",
			cel.Overload("labelin_map_string_list",
				[]*cel.Type{cel.MapType(cel.StringType, cel.StringType), cel.StringType, cel.ListType(cel.StringType)},
				cel.BoolType,
				cel.FunctionBinding(labelIn),
			),
		),

		// labelMatches(labels, key, pattern) - check if label matches regex
		cel.Function("labelMatches",
			cel.Overload("labelmatches_map_string_string",
				[]*cel.Type{cel.MapType(cel.StringType, cel.StringType), cel.StringType, cel.StringType},
				cel.BoolType,
				cel.FunctionBinding(labelMatches),
			),
		),

		// severityAtLeast(severity, minimum) - compare severity levels
		cel.Function("severityAtLeast",
			cel.Overload("severityatleast_string_string",
				[]*cel.Type{cel.StringType, cel.StringType},
				cel.BoolType,
				cel.BinaryBinding(severityAtLeast),
			),
		),

		// severityLevel(severity) - get numeric severity level
		cel.Function("severityLevel",
			cel.Overload("severitylevel_string",
				[]*cel.Type{cel.StringType},
				cel.IntType,
				cel.UnaryBinding(severityLevel),
			),
		),

		// lower(string) - convert to lowercase
		cel.Function("lower",
			cel.Overload("lower_string",
				[]*cel.Type{cel.StringType},
				cel.StringType,
				cel.UnaryBinding(lowerString),
			),
		),

		// upper(string) - convert to uppercase
		cel.Function("upper",
			cel.Overload("upper_string",
				[]*cel.Type{cel.StringType},
				cel.StringType,
				cel.UnaryBinding(upperString),
			),
		),

		// trim(string) - trim whitespace
		cel.Function("trim",
			cel.Overload("trim_string",
				[]*cel.Type{cel.StringType},
				cel.StringType,
				cel.UnaryBinding(trimString),
			),
		),

		// split(string, separator) - split string into list
		cel.Function("split",
			cel.Overload("split_string_string",
				[]*cel.Type{cel.StringType, cel.StringType},
				cel.ListType(cel.StringType),
				cel.BinaryBinding(splitString),
			),
		),

		// join(list, separator) - join list into string
		cel.Function("join",
			cel.Overload("join_list_string",
				[]*cel.Type{cel.ListType(cel.StringType), cel.StringType},
				cel.StringType,
				cel.BinaryBinding(joinList),
			),
		),
	}
}

// ProgramOptions implements cel.Library.
func (c *customFunctions) ProgramOptions() []cel.ProgramOption {
	return nil
}

// containsListString checks if a list contains a string.
func containsListString(lhs, rhs ref.Val) ref.Val {
	list, ok := lhs.(traits.Lister)
	if !ok {
		return types.Bool(false)
	}

	item, ok := rhs.(types.String)
	if !ok {
		return types.Bool(false)
	}

	it := list.Iterator()
	for it.HasNext() == types.True {
		elem := it.Next()
		if str, ok := elem.(types.String); ok {
			if str == item {
				return types.Bool(true)
			}
		}
	}

	return types.Bool(false)
}

// containsMapKey checks if a map contains a key.
func containsMapKey(lhs, rhs ref.Val) ref.Val {
	m, ok := lhs.(traits.Mapper)
	if !ok {
		return types.Bool(false)
	}

	key, ok := rhs.(types.String)
	if !ok {
		return types.Bool(false)
	}

	val := m.Get(key)
	if types.IsError(val) || val.Type() == types.ErrType {
		return types.Bool(false)
	}

	return types.Bool(true)
}

// matchesRegex performs a regex match.
func matchesRegex(lhs, rhs ref.Val) ref.Val {
	str, ok := lhs.(types.String)
	if !ok {
		return types.Bool(false)
	}

	pattern, ok := rhs.(types.String)
	if !ok {
		return types.Bool(false)
	}

	re, err := regexp.Compile(string(pattern))
	if err != nil {
		return types.Bool(false)
	}

	return types.Bool(re.MatchString(string(str)))
}

// startsWithString checks if string starts with prefix.
func startsWithString(lhs, rhs ref.Val) ref.Val {
	str, ok := lhs.(types.String)
	if !ok {
		return types.Bool(false)
	}

	prefix, ok := rhs.(types.String)
	if !ok {
		return types.Bool(false)
	}

	return types.Bool(strings.HasPrefix(string(str), string(prefix)))
}

// endsWithString checks if string ends with suffix.
func endsWithString(lhs, rhs ref.Val) ref.Val {
	str, ok := lhs.(types.String)
	if !ok {
		return types.Bool(false)
	}

	suffix, ok := rhs.(types.String)
	if !ok {
		return types.Bool(false)
	}

	return types.Bool(strings.HasSuffix(string(str), string(suffix)))
}

// getLabelWithDefault gets a label value with a default.
func getLabelWithDefault(args ...ref.Val) ref.Val {
	if len(args) != 3 {
		return types.String("")
	}

	m, ok := args[0].(traits.Mapper)
	if !ok {
		return args[2]
	}

	key, ok := args[1].(types.String)
	if !ok {
		return args[2]
	}

	val := m.Get(key)
	if types.IsError(val) || val.Type() == types.ErrType {
		return args[2]
	}

	return val
}

// labelEquals checks if a label equals a value.
func labelEquals(args ...ref.Val) ref.Val {
	if len(args) != 3 {
		return types.Bool(false)
	}

	m, ok := args[0].(traits.Mapper)
	if !ok {
		return types.Bool(false)
	}

	key, ok := args[1].(types.String)
	if !ok {
		return types.Bool(false)
	}

	expected, ok := args[2].(types.String)
	if !ok {
		return types.Bool(false)
	}

	val := m.Get(key)
	if types.IsError(val) || val.Type() == types.ErrType {
		return types.Bool(false)
	}

	actual, ok := val.(types.String)
	if !ok {
		return types.Bool(false)
	}

	return types.Bool(actual == expected)
}

// labelIn checks if a label value is in a list.
func labelIn(args ...ref.Val) ref.Val {
	if len(args) != 3 {
		return types.Bool(false)
	}

	m, ok := args[0].(traits.Mapper)
	if !ok {
		return types.Bool(false)
	}

	key, ok := args[1].(types.String)
	if !ok {
		return types.Bool(false)
	}

	list, ok := args[2].(traits.Lister)
	if !ok {
		return types.Bool(false)
	}

	val := m.Get(key)
	if types.IsError(val) || val.Type() == types.ErrType {
		return types.Bool(false)
	}

	actual, ok := val.(types.String)
	if !ok {
		return types.Bool(false)
	}

	it := list.Iterator()
	for it.HasNext() == types.True {
		elem := it.Next()
		if str, ok := elem.(types.String); ok {
			if str == actual {
				return types.Bool(true)
			}
		}
	}

	return types.Bool(false)
}

// labelMatches checks if a label matches a regex.
func labelMatches(args ...ref.Val) ref.Val {
	if len(args) != 3 {
		return types.Bool(false)
	}

	m, ok := args[0].(traits.Mapper)
	if !ok {
		return types.Bool(false)
	}

	key, ok := args[1].(types.String)
	if !ok {
		return types.Bool(false)
	}

	pattern, ok := args[2].(types.String)
	if !ok {
		return types.Bool(false)
	}

	val := m.Get(key)
	if types.IsError(val) || val.Type() == types.ErrType {
		return types.Bool(false)
	}

	actual, ok := val.(types.String)
	if !ok {
		return types.Bool(false)
	}

	re, err := regexp.Compile(string(pattern))
	if err != nil {
		return types.Bool(false)
	}

	return types.Bool(re.MatchString(string(actual)))
}

// severityLevel returns the numeric level for a severity string.
func severityLevel(val ref.Val) ref.Val {
	sev, ok := val.(types.String)
	if !ok {
		return types.Int(0)
	}

	return types.Int(severityToLevel(string(sev)))
}

// severityAtLeast checks if severity is at least a minimum level.
func severityAtLeast(lhs, rhs ref.Val) ref.Val {
	sev, ok := lhs.(types.String)
	if !ok {
		return types.Bool(false)
	}

	min, ok := rhs.(types.String)
	if !ok {
		return types.Bool(false)
	}

	sevLevel := severityToLevel(string(sev))
	minLevel := severityToLevel(string(min))

	return types.Bool(sevLevel >= minLevel)
}

// severityToLevel converts a severity string to a numeric level.
func severityToLevel(severity string) int64 {
	switch strings.ToLower(severity) {
	case "critical", "fatal", "p1":
		return 5
	case "high", "error", "p2":
		return 4
	case "warning", "warn", "medium", "p3":
		return 3
	case "info", "low", "p4":
		return 2
	case "debug", "p5":
		return 1
	default:
		return 0
	}
}

// lowerString converts a string to lowercase.
func lowerString(val ref.Val) ref.Val {
	str, ok := val.(types.String)
	if !ok {
		return types.String("")
	}
	return types.String(strings.ToLower(string(str)))
}

// upperString converts a string to uppercase.
func upperString(val ref.Val) ref.Val {
	str, ok := val.(types.String)
	if !ok {
		return types.String("")
	}
	return types.String(strings.ToUpper(string(str)))
}

// trimString trims whitespace from a string.
func trimString(val ref.Val) ref.Val {
	str, ok := val.(types.String)
	if !ok {
		return types.String("")
	}
	return types.String(strings.TrimSpace(string(str)))
}

// splitString splits a string by a separator.
func splitString(lhs, rhs ref.Val) ref.Val {
	str, ok := lhs.(types.String)
	if !ok {
		return types.NewStringList(types.DefaultTypeAdapter, []string{})
	}

	sep, ok := rhs.(types.String)
	if !ok {
		return types.NewStringList(types.DefaultTypeAdapter, []string{})
	}

	parts := strings.Split(string(str), string(sep))
	return types.NewStringList(types.DefaultTypeAdapter, parts)
}

// joinList joins a list into a string.
func joinList(lhs, rhs ref.Val) ref.Val {
	list, ok := lhs.(traits.Lister)
	if !ok {
		return types.String("")
	}

	sep, ok := rhs.(types.String)
	if !ok {
		return types.String("")
	}

	var parts []string
	it := list.Iterator()
	for it.HasNext() == types.True {
		elem := it.Next()
		if str, ok := elem.(types.String); ok {
			parts = append(parts, string(str))
		}
	}

	return types.String(strings.Join(parts, string(sep)))
}
