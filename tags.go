package template

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

type TagHandler struct {
	Execute func(args *string, tpl *Template, ctx *Context) (*string, error)
	Ignore  func(args *string, tpl *Template) error
}

var Tags = map[string]*TagHandler{
	"if":        &TagHandler{Execute: tagIf, Ignore: tagIfIgnore},
	"else":      nil, // Only a placeholder for the (if|for)-statement
	"endif":     nil, // Only a placeholder for the if-statement
	"for":       &TagHandler{Execute: tagFor, Ignore: tagForIgnore},
	"endfor":    nil,
	"block":     &TagHandler{Execute: tagBlock}, // Needs no Ignore-function because nested-blocks aren't allowed
	"endblock":  nil,
	"extends":   &TagHandler{Execute: tagExtends},
	"include":   &TagHandler{Execute: tagInclude},
	"trim":      &TagHandler{Execute: tagTrim, Ignore: tagTrimIgnore},
	"endtrim":   nil,
	"remove":    &TagHandler{Execute: tagRemove, Ignore: tagRemoveIgnore},
	"endremove": nil,
	/*"catch": tagCatch, // catches any panics and prints them
	"endcatch": nil,*/

	/*"while":    tagWhile,
	"endwhile": nil,
	"set":      tagSet,*/
}

type compareFunc func(interface{}, interface{}) bool

var compMap = map[string]compareFunc{
	"==": func(a, b interface{}) bool {
		return a == b
	},
	"!=": func(a, b interface{}) bool {
		return a != b
	},
	"<>": func(a, b interface{}) bool {
		return a != b
	},
	"&&": func(a, b interface{}) bool {
		ab, is_bool := a.(bool)
		if !is_bool {
			fmt.Printf("Warning: %v (%T) is not a bool!\n", a, a)
			return false
		}
		bb, is_bool := b.(bool)
		if !is_bool {
			fmt.Printf("Warning: %v (%T) is not a bool!\n", b, b)
			return false
		}
		res := ab && bb
		return res
	},
	"||": func(a, b interface{}) bool {
		ab, is_bool := a.(bool)
		if !is_bool {
			fmt.Printf("Warning: %v (%T) is not a bool!\n", a, a)
			return false
		}
		bb, is_bool := b.(bool)
		if !is_bool {
			fmt.Printf("Warning: %v (%T) is not a bool!\n", b, b)
			return false
		}
		return ab || bb
	},
	">=": func(a, b interface{}) bool {
		switch av := a.(type) {
		case int:
			switch bv := b.(type) {
			case int:
				return av >= bv
			case float64:
				return float64(av) >= bv
			}
		case float64:
			switch bv := b.(type) {
			case int:
				return av >= float64(bv)
			case float64:
				return av >= bv
			}
		default:
			fmt.Printf("Warning! Invalid (type) comparison between '%v' (%T) and '%v' (%T).\n", a, a, b, b)
		}
		return false
	},
	"<=": func(a, b interface{}) bool {
		switch av := a.(type) {
		case int:
			switch bv := b.(type) {
			case int:
				return av <= bv
			case float64:
				return float64(av) <= bv
			}
		case float64:
			switch bv := b.(type) {
			case int:
				return av <= float64(bv)
			case float64:
				return av <= bv
			}
		default:
			fmt.Printf("Warning! Invalid (type) comparison between '%v' (%T) and '%v' (%T).\n", a, a, b, b)
		}
		return false
	},
	"<": func(a, b interface{}) bool {
		switch av := a.(type) {
		case int:
			switch bv := b.(type) {
			case int:
				return av < bv
			case float64:
				return float64(av) < bv
			}
		case float64:
			switch bv := b.(type) {
			case int:
				return av < float64(bv)
			case float64:
				return av < bv
			}
		default:
			fmt.Printf("Warning! Invalid (type) comparison between '%v' (%T) and '%v' (%T).\n", a, a, b, b)
		}
		return false
	},
	">": func(a, b interface{}) bool {
		switch av := a.(type) {
		case int:
			switch bv := b.(type) {
			case int:
				return av > bv
			case float64:
				return float64(av) > bv
			default:
				fmt.Printf("Warning! Invalid (type) comparison between '%v' (%T) and '%v' (%T).\n", a, a, b, b)
			}
		case float64:
			switch bv := b.(type) {
			case int:
				return av > float64(bv)
			case float64:
				return av > bv
			default:
				fmt.Printf("Warning! Invalid (type) comparison between '%v' (%T) and '%v' (%T).\n", a, a, b, b)
			}
		default:
			fmt.Printf("Warning! Invalid (type) comparison between '%v' (%T) and '%v' (%T).\n", a, a, b, b)
		}
		return false
	},
}

func containsAnyOperator(where string, ops ...string) bool {
	// TODO: Respect strings which contains operators/comparables. :D I've to 
	// develop a more intelligent way of "strings.Contains" and have to
	// replace this function. Same goes for the expressions (see comment in expr.go)
	for _, op := range ops {
		if strings.Contains(where, op) {
			return true
		}
	}
	return false
}

func evalOperation(where string, ctx *Context, ops ...string) (bool, error) {
	// Determine which operation to execute
	var op string

	// TODO: Respect strings which contains operators/comparables. :D I've to 
	// develop a more intelligent way of "strings.Contains" and have to
	// replace this function. Same goes for the expressions (see comment in expr.go)
	for _, _op := range ops {
		if strings.Contains(where, _op) {
			op = _op
			break
		}
	}

	args := strings.SplitN(where, op, 2)
	if len(args) != 2 {
		return false, errors.New(fmt.Sprintf("%s-operator must have 2 operands (like X and Y).", op))
	}

	e1, err1 := evalCondArg(ctx, &args[0])
	if err1 != nil {
		return false, err1
	}

	e2, err2 := evalCondArg(ctx, &args[1])
	if err2 != nil {
		return false, err2
	}

	op_func, has_op := compMap[op]
	if !has_op {
		return false, errors.New(fmt.Sprintf("Operator-handler for '%s' not found.", op))
	}

	return op_func(e1, e2), nil
}

func evalCondArg(ctx *Context, in *string) (interface{}, error) {
	switch {
	// and/or operator (1st class)
	case containsAnyOperator(*in, "&&", "||"):
		result, err := evalOperation(*in, ctx, "&&", "||")
		if err != nil {
			return false, err
		}
		return result, nil

	// ==, !=, <>, >=, <= operator (2nd class)
	case containsAnyOperator(*in, "==", "!=", "<>", ">=", "<=", ">", "<"):
		result, err := evalOperation(*in, ctx, "==", "!=", "<>", ">=", "<=", ">", "<")
		if err != nil {
			return false, err
		}
		return result, nil

	default:
		e, err := newExpr(in)
		if err != nil {
			return false, err
		}
		return e.evalValue(ctx)
	}

	panic("unreachable")
}

func tagIf(args *string, tpl *Template, ctx *Context) (*string, error) {
	renderedStrings := make([]string, 0, len(tpl.nodes)-tpl.node_pos)

	*args = strings.TrimSpace(*args)
	if len(*args) == 0 {
		return nil, errors.New("If-argument is empty.")
	}

	evaled, err := evalCondArg(ctx, args)
	if err != nil {
		return nil, err
	}

	res_bool, is_bool := evaled.(bool)
	if !is_bool {
		// {% if x %}
		// Anything evals to TRUE which is DIFFER from the type's default value!
		res_bool = reflect.Zero(reflect.TypeOf(evaled)).Interface() != evaled
	}

	if res_bool {
		node, str_items, err := tpl.executeUntilAnyTagNode(ctx, "else", "endif")
		if err != nil {
			return nil, err
		}
		renderedStrings = append(renderedStrings, (*str_items)...)

		if node.tagname == "else" { // There's an else-block, skip it
			_, err := tpl.ignoreUntilAnyTagNode("endif")
			if err != nil {
				return nil, err
			}
		}
	} else {
		node, err := tpl.ignoreUntilAnyTagNode("else", "endif")
		if err != nil {
			return nil, err
		}

		if node.tagname == "else" {
			_, str_items, err := tpl.executeUntilAnyTagNode(ctx, "endif")
			if err != nil {
				return nil, err
			}
			renderedStrings = append(renderedStrings, (*str_items)...)
		}
	}

	outputString := strings.Join(renderedStrings, "")
	return &outputString, nil
}

func tagIfIgnore(args *string, tpl *Template) error {
	tn, err := tpl.ignoreUntilAnyTagNode("else", "endif")
	if err != nil {
		return err
	}
	if tn.tagname == "else" {
		_, err := tpl.ignoreUntilAnyTagNode("endif")
		if err != nil {
			return err
		}
	}
	return nil
}

type forContext struct {
	Counter  int
	Counter1 int
	Max      int
	Max1     int
	First    bool
	Last     bool
}

func tagFor(args *string, tpl *Template, ctx *Context) (*string, error) {
	var renderedStrings []string

	// TODO: Replace strings.Contains by a more intelligent function (see comment above as well)
	if strings.Contains(*args, "in") {
		// <varname> in <slice/array/string/map>
		// TODO: Update context with "forloop"-struct every loop round
		args := strings.SplitN(*args, "in", 2)
		if len(args) != 2 {
			return nil, errors.New("When using 'in' in for-loop, it must use the following syntax: <varname> in <array/slice/string/map>")
		}
		varname := strings.TrimSpace(args[0])
		e, err := newExpr(&args[1])
		if err != nil {
			return nil, err
		}
		value, err := e.evalValue(ctx)
		if err != nil {
			return nil, err
		}
		rv := reflect.ValueOf(value)
		switch rv.Kind() {
		case reflect.Slice, reflect.Array, reflect.String, reflect.Map:
			// Iterate through slice/array

			if rv.Len() > 0 {
				// Prepare renderedStrings
				renderedStrings = make([]string, 0, (len(tpl.nodes)-tpl.node_pos)*rv.Len())

				// If map, get all keys
				var map_items []reflect.Value
				if rv.Kind() == reflect.Map {
					map_items = rv.MapKeys()
				}

				// Create for-context
				forCtx := &forContext{
					Max:      rv.Len() - 1,
					Max1:     rv.Len(),
					Counter1: 1,
					First:    true,
				}

				// Check if this is a nested loop (3rd grade)
				// If so, add to forloops.
				forloops, has_forloops := (*ctx)["forloops"]
				if has_forloops {
					forloops = append(forloops.([]*forContext), forCtx)
					(*ctx)["forloops"] = forloops // Pointer might have been changed, this is why we set it again 
				} else {
					// Check if this is a nested loop (2nd grade)
					// If so, populate forloops.
					_forloop, has_forloop := (*ctx)["forloop"]
					if has_forloop {
						// Create forloops and add prev and current context to it
						has_forloops = true
						forloops = []*forContext{_forloop.(*forContext), forCtx}
						(*ctx)["forloops"] = forloops
					}
				}

				// Do the loops
				starter_pos := tpl.node_pos
				for i := 0; i < rv.Len(); i++ {
					// Handle each type separately
					var item interface{}
					switch rv.Kind() {
					case reflect.Slice, reflect.Array:
						item = rv.Index(i).Interface()
						(*ctx)[varname] = item
					case reflect.Map:
						// Create special Context struct for a map
						(*ctx)[varname] = struct {
							Key   interface{}
							Value interface{}
						}{
							Key:   map_items[i].Interface(),
							Value: rv.MapIndex(map_items[i]).Interface(),
						}
					case reflect.String:
						item = rv.Interface().(string)[i : i+1]
						(*ctx)[varname] = item
					}
					tpl.node_pos = starter_pos

					// Populate and update for-context
					if i == 1 {
						forCtx.First = false
					}
					if i == rv.Len()-1 {
						// Last item reached
						forCtx.Last = true
					}

					(*ctx)["forloop"] = forCtx
					(*ctx)["forcounter"] = i
					(*ctx)["forcounter1"] = i + 1

					// Execute for-body
					tn, str_items, err := tpl.executeUntilAnyTagNode(ctx, "else", "endfor")
					if err != nil {
						return nil, err
					}
					if tn.tagname == "else" {
						// Skip else since it's not relevant
						tpl.ignoreUntilAnyTagNode("endfor")
					}
					renderedStrings = append(renderedStrings, (*str_items)...)

					// Increase counters
					forCtx.Counter++
					forCtx.Counter1++
				}

				// Remove for-context
				delete(*ctx, varname)
				delete(*ctx, "forloop")
				delete(*ctx, "forcounter")
				delete(*ctx, "forcounter1")

				// Check for nested, if so, remove myself from forloops
				if has_forloops {
					forloops = (forloops.([]*forContext))[:len(forloops.([]*forContext))-1]
					(*ctx)["forloops"] = forloops
				}

				// Check whether forloops can be removed
				if has_forloops && len(forloops.([]*forContext)) == 0 {
					delete(*ctx, "forloops")
				}
			} else {
				// Zero executions, directly execute else or go to endfor
				tn, err := tpl.ignoreUntilAnyTagNode("else", "endfor")
				if err != nil {
					return nil, err
				}
				if tn.tagname == "else" {
					// Execute empty block
					_, str_items, err := tpl.executeUntilAnyTagNode(ctx, "endfor")
					if err != nil {
						return nil, err
					}
					renderedStrings = append(renderedStrings, (*str_items)...)
				}
			}
		default:
			return nil, errors.New("For-loop 'in'-operator can onl be used for slices/arrays/strings/maps.")
		}
	} else {
		// try to evaluate the argument, and run in X times if it evaluates to an integer
		e, err := newExpr(args)
		if err != nil {
			return nil, err
		}
		value, err := e.evalValue(ctx)
		if err != nil {
			return nil, err
		}

		// If value is an integer, iterate X times.
		if rng, is_int := value.(int); is_int {
			if rng > 0 {
				// Prepare renderedStrings
				renderedStrings = make([]string, 0, (len(tpl.nodes)-tpl.node_pos)*rng)

				// Create for-context
				forCtx := &forContext{
					Max:      rng - 1,
					Max1:     rng,
					Counter1: 1,
					First:    true,
				}

				// Check if this is a nested loop (3rd grade)
				// If so, add to forloops.
				forloops, has_forloops := (*ctx)["forloops"]
				if has_forloops {
					forloops = append(forloops.([]*forContext), forCtx)
					(*ctx)["forloops"] = forloops // Pointer might have been changed, this is why we set it again 
				} else {
					// Check if this is a nested loop (2nd grade)
					// If so, populate forloops.
					_forloop, has_forloop := (*ctx)["forloop"]
					if has_forloop {
						// Create forloops and add prev and current context to it
						has_forloops = true
						forloops = []*forContext{_forloop.(*forContext), forCtx}
						(*ctx)["forloops"] = forloops
					}
				}

				// Do the loops
				starter_pos := tpl.node_pos
				for i := 0; i < rng; i++ {
					tpl.node_pos = starter_pos

					// Populate and update for-context
					if i == 1 {
						forCtx.First = false
					}
					if i == rng-1 {
						// Last item reached
						forCtx.Last = true
					}

					(*ctx)["forloop"] = forCtx // overwrite current forloop-context
					(*ctx)["forcounter"] = i
					(*ctx)["forcounter1"] = i + 1

					// Execute for-body
					tn, str_items, err := tpl.executeUntilAnyTagNode(ctx, "else", "endfor")
					if err != nil {
						return nil, err
					}
					if tn.tagname == "else" {
						// Skip else since it's not relevant
						tpl.ignoreUntilAnyTagNode("endfor")
					}
					renderedStrings = append(renderedStrings, (*str_items)...)

					// Increase counters
					forCtx.Counter++
					forCtx.Counter1++
				}

				// Remove for-context
				delete(*ctx, "forloop")
				delete(*ctx, "forcounter")
				delete(*ctx, "forcounter1")

				// Check for nested, if so, remove myself from forloops
				if has_forloops {
					forloops = (forloops.([]*forContext))[:len(forloops.([]*forContext))-1]
					(*ctx)["forloops"] = forloops
				}

				// Check whether forloops can be removed
				if has_forloops && len(forloops.([]*forContext)) == 0 {
					delete(*ctx, "forloops")
				}
			} else {
				// Zero executions, directly execute else or go to endfor
				tn, err := tpl.ignoreUntilAnyTagNode("else", "endfor")
				if err != nil {
					return nil, err
				}
				if tn.tagname == "else" {
					// Execute empty block
					_, str_items, err := tpl.executeUntilAnyTagNode(ctx, "endfor")
					if err != nil {
						return nil, err
					}
					renderedStrings = append(renderedStrings, (*str_items)...)
				}
			}
		} else {
			return nil, errors.New(fmt.Sprintf("For-loop error: Cannot iterate over '%v'.", *args))
		}
	}

	outputString := strings.Join(renderedStrings, "")
	return &outputString, nil
}

func tagForIgnore(args *string, tpl *Template) error {
	tn, err := tpl.ignoreUntilAnyTagNode("else", "endfor")
	if err != nil {
		return err
	}
	if tn.tagname == "else" {
		_, err := tpl.ignoreUntilAnyTagNode("endfor")
		if err != nil {
			return err
		}
	}
	return nil
}

func tagBlock(args *string, tpl *Template, ctx *Context) (*string, error) {
	renderedStrings := make([]string, 0, len(tpl.nodes)-tpl.node_pos)

	// TODO: Prevent nested block-tags

	// Check whether we replace this block by a internal Context or 
	// if we render the default content
	child_block, has_childblock := tpl.internal_context[fmt.Sprintf("block_%s", *args)]
	if has_childblock {
		// Use the prerendered child's data as output
		str, is_string := child_block.(*string)
		if !is_string {
			panic("Internal error; internal block string is NOT a string. Please report this issue.")
		}
		// Now we have to ignore the default block
		_, err := tpl.ignoreUntilAnyTagNode("endblock")
		if err != nil {
			return nil, err
		}

		// Return the prerendered data
		return str, nil
	}

	// Execute default nodes
	_, str_items, err := tpl.executeUntilAnyTagNode(ctx, "endblock")
	if err != nil {
		return nil, err
	}
	renderedStrings = append(renderedStrings, (*str_items)...)

	outputString := strings.Join(renderedStrings, "")
	return &outputString, nil
}

func tagTrim(args *string, tpl *Template, ctx *Context) (*string, error) {
	renderedStrings := make([]string, 0, len(tpl.nodes)-tpl.node_pos)

	// Execute content
	_, str_items, err := tpl.executeUntilAnyTagNode(ctx, "endtrim")
	if err != nil {
		return nil, err
	}
	renderedStrings = append(renderedStrings, (*str_items)...)

	outputString := strings.TrimSpace(strings.Join(renderedStrings, ""))
	return &outputString, nil
}

func tagTrimIgnore(args *string, tpl *Template) error {
	_, err := tpl.ignoreUntilAnyTagNode("endtrim")
	if err != nil {
		return err
	}
	return nil
}

func tagRemove(args *string, tpl *Template, ctx *Context) (*string, error) {
	renderedStrings := make([]string, 0, len(tpl.nodes)-tpl.node_pos)

	// Execute content
	_, str_items, err := tpl.executeUntilAnyTagNode(ctx, "endremove")
	if err != nil {
		return nil, err
	}
	renderedStrings = append(renderedStrings, (*str_items)...)
	outputString := strings.Join(renderedStrings, "")

	// Parse args {% remove "abc","def","ghj" %}
	patterns := *splitArgs(args, ",")
	if len(patterns) == 0 {
		// default patterns (spaces, tabs, new lines)
		patterns = []string{"\" \"", "\"\t\"", "\"\n\"", "\"\r\""}
	}

	// Do remove all the patterns
	for _, pattern := range patterns {
		e, err := newExpr(&pattern)
		if err != nil {
			return nil, err
		}
		evaledPattern, err := e.evalString(ctx)
		if err != nil {
			return nil, err
		}
		outputString = strings.Replace(outputString, *evaledPattern, "", -1)
	}

	return &outputString, nil
}

func tagRemoveIgnore(args *string, tpl *Template) error {
	_, err := tpl.ignoreUntilAnyTagNode("endremove")
	if err != nil {
		return err
	}
	return nil
}

func tagExtends(args *string, tpl *Template, ctx *Context) (*string, error) {
	// Extends executes the base template and passes the blocks via Context 

	// Example: {% extends "base.html" abc=<expr> ghi=<expr> ... %}
	_args := strings.Split(*args, " ")
	if len(_args) <= 0 {
		return nil, errors.New("Please provide at least a filename to extend from.")
	}
	e, err := newExpr(&_args[0])
	if err != nil {
		return nil, err
	}
	name, err := e.evalString(ctx)
	if err != nil {
		return nil, err
	}
	//raw_context := _args[1:] // TODO

	// Create new template
	if tpl.locator == nil {
		panic(fmt.Sprintf("Please provide a template locator to lookup template '%v'.", *name))
	}

	base_tpl_content, err := tpl.locator(name)
	if err != nil {
		return nil, err
	}
	
	// TODO: Do the pre-rendering (FromString) in the parent's FromString(), just do the execution here.
	base_tpl, err := FromString(*name, base_tpl_content, tpl.locator)
	if err != nil {
		return nil, err
	}

	// Set base' internal context to my one so it can access all the blocks
	base_tpl.setInternalContext(&tpl.internal_context)

	// Execute every 'block' and store it's result as "block_%s" in the internal Context
	for {
		node, err := tpl.ignoreUntilAnyTagNode("block")
		if err != nil {
			// No block left
			break
		}
		blockname := node.tagargs
		node, str_items, err := tpl.executeUntilAnyTagNode(ctx, "endblock")
		if err != nil {
			return nil, err
		}
		rendered_string := strings.Join(*str_items, "")
		tpl.internal_context[fmt.Sprintf("block_%s", blockname)] = &rendered_string
	}

	return base_tpl.Execute(ctx)
}

func tagInclude(args *string, tpl *Template, ctx *Context) (*string, error) {
	// Includes a template and executes it 

	// Example: {% include "base.html" abc=<expr> ghi=<expr> ... %}
	_args := strings.Split(*args, " ")
	if len(_args) <= 0 {
		return nil, errors.New("Please provide at least a filename to extend from.")
	}
	e, err := newExpr(&_args[0])
	if err != nil {
		return nil, err
	}
	name, err := e.evalString(ctx)
	if err != nil {
		return nil, err
	}
	//raw_context := _args[1:]  // TODO

	// Create new template
	if tpl.locator == nil {
		panic(fmt.Sprintf("Please provide a template locator to lookup template '%v'.", *name))
	}

	base_tpl_content, err := tpl.locator(name)
	if err != nil {
		return nil, err
	}
	
	// TODO: Do the pre-rendering (FromString) in the parent's FromString(), just do the execution here. 
	base_tpl, err := FromString(*name, base_tpl_content, tpl.locator)
	if err != nil {
		return nil, err
	}

	return base_tpl.Execute(ctx)
}
