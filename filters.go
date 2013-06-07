package pongo

// TODO: Add context-sensitive filters (so they know their location, e.g. for 
// context-sensitive escaping within javascript <-> normal body html.)

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type FilterFunc func(interface{}, []interface{}, *FilterChainContext) (interface{}, error)

type FilterChainContext struct {
	// Store what you want along the filter chain. Every filter has access to this store.
	Store           map[string]interface{}
	applied_filters []string
}

func (ctx *FilterChainContext) HasVisited(names ...string) bool {
	for _, filter := range ctx.applied_filters {
		for _, name := range names {
			if filter == name {
				return true
			}
		}
	}
	return false
}

func (ctx *FilterChainContext) visitFilter(name string) {
	ctx.applied_filters = append(ctx.applied_filters, name)
}

var Filters = map[string]FilterFunc{
	"safe":        filterSafe,
	"unsafe":      nil, // It will not be called, just added to visited filters (applied_filters)
	"lower":       filterLower,
	"upper":       filterUpper,
	"capitalize":  filterCapitalize,
	"default":     filterDefault,
	"trim":        filterTrim,
	"length":      filterLength,
	"join":        filterJoin,
	"striptags":   filterStriptags,
	"time_format": filterTimeFormat,
	"floatformat": filterFloatFormat,

	/* TODO:
	- verbatim
	- ...
	*/
}

func newFilterChainContext() *FilterChainContext {
	return &FilterChainContext{
		applied_filters: make([]string, 0, 5),
	}
}

func filterSafe(value interface{}, args []interface{}, ctx *FilterChainContext) (interface{}, error) {
	if ctx.HasVisited("unsafe", "safe") {
		// If "unsafe" or "safe" were already applied to the value
		// don't do it (again, in case of "safe")
		return value, nil
	}

	str, is_str := value.(string)
	if !is_str {
		// We don't have to safe non-strings
		return value, nil
	}

	output := strings.Replace(str, "&", "&amp;", -1)
	output = strings.Replace(output, ">", "&gt;", -1)
	output = strings.Replace(output, "<", "&lt;", -1)

	return output, nil
}

func filterLower(value interface{}, args []interface{}, ctx *FilterChainContext) (interface{}, error) {
	str, is_str := value.(string)
	if !is_str {
		return nil, errors.New(fmt.Sprintf("%v (%T) is not of type string", value, value))
	}
	return strings.ToLower(str), nil
}

func filterTimeFormat(value interface{}, args []interface{}, ctx *FilterChainContext) (interface{}, error) {
	t, is_time := value.(time.Time)
	if !is_time {
		return nil, errors.New(fmt.Sprintf("%v (%T) is not of type string", value, value))
	}

	arg := args[0]
	if arg == nil {
		return nil, errors.New("time_format requires you pass a format.")
	}

	format, is_string := arg.(string)
	if !is_string {
		return nil, errors.New(fmt.Sprintf("time_format's format must be a string. %v (%T) passed.", format, format))
	}

	return t.Format(format), nil
}

func filterUpper(value interface{}, args []interface{}, ctx *FilterChainContext) (interface{}, error) {
	str, is_str := value.(string)
	if !is_str {
		return nil, errors.New(fmt.Sprintf("%v (%T) is not of type string", value, value))
	}
	return strings.ToUpper(str), nil
}

func filterCapitalize(value interface{}, args []interface{}, ctx *FilterChainContext) (interface{}, error) {
	str, is_str := value.(string)
	if !is_str {
		return nil, errors.New(fmt.Sprintf("%v (%T) is not of type string", value, value))
	}
	return strings.Title(str), nil
}

func filterTrim(value interface{}, args []interface{}, ctx *FilterChainContext) (interface{}, error) {
	str, is_str := value.(string)
	if !is_str {
		return nil, errors.New(fmt.Sprintf("%v (%T) is not of type string", value, value))
	}
	return strings.TrimSpace(str), nil
}

func filterLength(value interface{}, args []interface{}, ctx *FilterChainContext) (interface{}, error) {
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Slice, reflect.Array, reflect.String, reflect.Map:
		return rv.Len(), nil
	default:
		return nil, errors.New(fmt.Sprintf("Cannot determine length from type %T ('%v').", value, value))
	}
	panic("unreachable")
}

func filterJoin(value interface{}, args []interface{}, ctx *FilterChainContext) (interface{}, error) {
	if len(args) != 1 {
		return nil, errors.New("Please provide a separator")
	}
	sep, is_string := args[0].(string)
	if !is_string {
		return nil, errors.New(fmt.Sprintf("Separator must be of type string, not %T ('%v')", args[0], args[0]))
	}

	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		items := make([]string, 0, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			items = append(items, fmt.Sprintf("%v", rv.Index(i).Interface()))
		}
		return strings.Join(items, sep), nil
	default:
		return nil, errors.New(fmt.Sprintf("Cannot join variable of type %T ('%v').", value, value))
	}
	panic("unreachable")
}

func filterStriptags(value interface{}, args []interface{}, ctx *FilterChainContext) (interface{}, error) {
	str, is_str := value.(string)
	if !is_str {
		return nil, errors.New(fmt.Sprintf("%v is not of type string", value))
	}

	if len(args) > 1 {
		return nil, errors.New("Please provide a comma-seperated string with tags (or no string to remove all tags).")
	}

	if len(args) == 1 {
		taglist, is_string := args[0].(string)
		if !is_string {
			return nil, errors.New(fmt.Sprintf("Taglist must be a string, not %T ('%v')", args[0], args[0]))
		}

		tags := strings.Split(taglist, ",")

		for _, tag := range tags {
			re := regexp.MustCompile(fmt.Sprintf("</?%s/?>", tag))
			str = re.ReplaceAllString(str, "")
		}
	} else {
		re := regexp.MustCompile("<[^>]*?>")
		str = re.ReplaceAllString(str, "")
	}

	return strings.TrimSpace(str), nil
}

func filterDefault(value interface{}, args []interface{}, ctx *FilterChainContext) (interface{}, error) {
	// Use reflect to check against zero() of type

	if len(args) != 1 {
		return nil, errors.New("Default filter takes only one argument")
	}

	if reflect.Zero(reflect.TypeOf(value)).Interface() == value {
		return args[0], nil
	}

	return value, nil
}

/*
	Filter for formatting floats. The filter closely follows Django's implementation.

	Examples:

		When used without an argument, it rounds the float to 1 decimal place, but only if there's a decimal point to be displayed:

		{{ 34.23234|floatformat }} displays 34.2
		{{ 34.00000|floatformat }} displays 34
		{{ 34.26000|floatformat }} displays 34.3

		When used with an integer parameter, it rounds the float to that number of decimals. No trimming of zeros occurs.

		{{ 34.23234|floatformat:3 }} displays 34.232
		{{ 34.00000|floatformat:3 }} displays 34.000
		{{ 34.26000|floatformat:3 }} displays 34.260

		"0" rounds to the nearest integer.

		{{ 34.23234|floatformat:"0" }} displays 34
		{{ 34.00000|floatformat:"0" }} displays 34
		{{ 39.56000|floatformat:"0" }} displays 40

		A negative parameter rounds to that number of decimals, but only if necessary.

		{{ 34.23234|floatformat:"-3" }} displays 34.232
		{{ 34.00000|floatformat:"-3" }} displays 34
		{{ 34.26000|floatformat:"-3" }} displays 34.260

*/
func filterFloatFormat(value interface{}, args []interface{}, ctx *FilterChainContext) (interface{}, error) {

	// Value to format
	var floatValue float64
	switch val := value.(type) {
	case float32:
		floatValue = float64(val)
	case float64:
		floatValue = val
	default:
		return nil, errors.New("Illegal type for floatformat (only float32 and float64 are acceptable)")
	}

	// Default parameters
	decimals, trim := 1, true
	if len(args) > 1 {
		return nil, errors.New("Floatformat filter takes at most one argument")
	} else if len(args) == 1 {
		switch val := args[0].(type) {
		case int:
			decimals = val
			trim = false
		case string:
			var err error
			decimals, err = strconv.Atoi(val)
			if err != nil {
				return nil, errors.New(fmt.Sprintf("Illegal floatformat argument: %v", val))
			}
			if decimals <= 0 {
				decimals = -decimals
			} else {
				trim = false
			}
		default:
			return nil, errors.New(fmt.Sprintf("%v (%T) is not of type int or string", val, val))
		}
	}

	fmtFloat := strconv.FormatFloat(floatValue, 'f', decimals, 64)

	// Remove zeroes if they are unnecessary
	if trim {
		intVal := int(floatValue)
		if floatValue-float64(intVal) == 0 {
			fmtFloat = strconv.Itoa(intVal)
		}
	}
	return fmtFloat, nil
}
