package template

// TODO: Add context-sensitive filters (so they know their location, e.g. for 
// context-sensitive escaping within javascript <-> normal body html.)

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"
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
	"safe":       filterSafe,
	"unsafe":     nil, // It will not be called, just added to visited filters (applied_filters)
	"lower":      filterLower,
	"upper":      filterUpper,
	"capitalize": filterCapitalize,
	"default":    filterDefault,
	"trim":       filterTrim,
	"length":     filterLength,
	"join":       filterJoin,
	"striptags":  filterStriptags,

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

	output := fmt.Sprintf("%v", value)

	output = strings.Replace(output, "&", "&amp;", -1)
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
