package pongo

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

var exprIdentChecker = regexp.MustCompile("^[A-Za-z0-9_]+[A-Za-z0-9_.]*$")

type exprIdent string

type exprFilterFunc struct {
	name string
	fn   FilterFunc
	args []interface{}
}

// An expression represents an expression used in {{ }} or other situations like
// {% if name|lower .... %} where name|lower is the expression. 
type expr struct {
	raw string

	root      interface{}
	root_args []reflect.Value
	filters   []exprFilterFunc
	negate    bool
}

func resolvePointer(v reflect.Value) reflect.Value {
	if v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		e := v.Elem()
		if e.CanInterface() && e.IsValid() {
			return e
		}
		panic("not implemented - todo")
	}
	return v
}

func resolveIdent(name exprIdent, ctx *Context) (interface{}, error) {
	parts := strings.Split(string(name), ".")

	if len(parts) == 0 {
		return nil, errors.New("Identifier is emtpy")
	}

	// Get first item from context
	ctxname := parts[0]
	parts = parts[1:]

	var value interface{}

	content, has := (*ctx)[ctxname]
	if !has {
		// If the identifier is not found
		// TODO add error in strict mode
		// fmt.Printf("Identifier '%v' NOT found in context (assuming empty string), but continuing. Skipping any further specifier.\n", ctxname)
		return "", nil
	}
	unresolved_value := content // Is needed for receiver-bounded methods (pointer <-> value)
	value = resolvePointer(reflect.ValueOf(content)).Interface()

	for idx_specifier, raw_specifier := range parts {
		if len(strings.TrimSpace(raw_specifier)) == 0 {
			return nil, errors.New("Specifier is empty!")
		}

		specifier, err := convertTypeString(raw_specifier)
		if err != nil {
			fmt.Printf("Specifier '%v' not found (in '%s')\n", raw_specifier, string(name))
			return "", nil // TODO: Specifier not found? Return empty string. Maybe return an error in a future strict mode.
		}

		// Depending on the current value only a restrict subset of values is allowed:
		//    slice/array -> int (as an index)
		//    struct -> exported funcs + attributes 
		//    map -> get by key
		//    string -> int (index)
		rv := reflect.ValueOf(value)

		// Check for a method on this type and execute it if found
		attr, is_ident := specifier.(exprIdent)
		if is_ident && value != nil {
			m := reflect.ValueOf(unresolved_value).MethodByName(string(attr))
			if m.IsValid() {
				// Method found

				// Execute method, if there is one specifier following this method call
				// otherwise return method reference back to the caller to allow
				// a call with arguments

				if idx_specifier+1 < len(parts) {
					// Call the method to allow following the chain

					// First check whether the function needs an argument, if so
					// set the return value to an empty string
					// (this is due here are no arguments allowed)

					if m.Type().NumIn() > 0 {
						// Arguments required
						return "", nil
					}

					results := m.Call(nil) // No function arguments allowed
					if len(results) > 1 {
						return nil, errors.New(fmt.Sprintf("Method '%s' returns more than one value, this does not work.", string(attr)))
					}
					if len(results) == 0 {
						return "", nil
					}
					if !results[0].CanInterface() {
						return "", nil
					}
					unresolved_value = results[0].Interface()
					value = resolvePointer(results[0]).Interface()

					continue // Next specifier
				} else {
					// We're at the end of the chain, return the reference to the method
					return m, nil
				}
			}
		}

	sw:
		switch rv.Kind() {
		case reflect.Array, reflect.Slice:
			idx, is_int := specifier.(int)
			if !is_int {
				// No integer index is given, maybe we want access the index from the Context
				solved_ident, err := resolveIdent(exprIdent(raw_specifier), ctx)
				idx, is_int = solved_ident.(int)
				if err != nil || !is_int {
					fmt.Printf("If you want to access an array/slice, specifier ('%v') must be an integer (will be used as an index).\n", specifier)
					return "", nil
				}
			}
			if idx < 0 || idx >= rv.Len() { // out of range
				return "", nil
			}
			new_value := rv.Index(idx)
			if !new_value.IsValid() || !new_value.CanInterface() {
				return "", nil
			}
			unresolved_value = new_value
			value = resolvePointer(new_value).Interface()

		case reflect.String:
			// specifier must be an int (as index)
			idx, is_int := specifier.(int)
			if !is_int {
				// No integer index is given, maybe we want access the index from the Context
				solved_ident, err := resolveIdent(exprIdent(raw_specifier), ctx)
				idx, is_int = solved_ident.(int)
				if err != nil || !is_int {
					fmt.Printf("If you want to access a string, specifier ('%v') must be an integer (will be used as an index).\n", specifier)
					return "", nil
				}
			}
			str, is_str := value.(string)
			if !is_str { // Should never evaluate to true
				panic("internal error: detected reflect.String but type assertion to string failed")
			}
			if idx < 0 || idx >= len(str) { // out of range
				return "", nil
			}
			value = str[idx : idx+1]

		case reflect.Map:
			if rv.IsNil() { // Is map, == nil?
				return "", nil
			}

			// specifier must be a string
			attr, is_ident := specifier.(exprIdent)
			if !is_ident {
				fmt.Printf("If you want to access a map, specifier ('%v') must be a qualified identifier.\n", specifier)
				return "", nil
				//break sw
			}
			mi := rv.MapIndex(reflect.ValueOf(string(attr)))
			if !mi.IsValid() || !mi.CanInterface() {
				// Map key not found or not interfaceable

				// Maybe we want access the map via a key from the Context
				solved_ident, err := resolveIdent(exprIdent(raw_specifier), ctx)
				key, is_str := solved_ident.(string)

				if is_str {
					// We received a string from the Context, try this as a key for the map
					mi = rv.MapIndex(reflect.ValueOf(key))
				}

				if err != nil || !is_str || !mi.IsValid() || !mi.CanInterface() {
					return "", nil
				}
			}
			unresolved_value = mi
			value = resolvePointer(mi).Interface()

		case reflect.Struct:
			// specifier must be a string
			attr, is_ident := specifier.(exprIdent)
			if !is_ident {
				fmt.Printf("If you want to access a struct, specifier ('%v') must be a qualified identifier.\n", specifier)
				break sw
			}
			new_value := rv.FieldByName(string(attr))
			if !new_value.IsValid() || !new_value.CanInterface() {
				// Maybe we want access the struct via a key from the Context
				solved_ident, err := resolveIdent(exprIdent(raw_specifier), ctx)
				key, is_str := solved_ident.(string)

				if is_str {
					// We received a string from the Context, try this as a key for the struct
					new_value = rv.FieldByName(key)
				}

				if err != nil || !is_str || !new_value.IsValid() || !new_value.CanInterface() {
					// If new value is not valid (because it does not exist) or is not exported (can not being interfaced)
					// return an empty string
					return "", nil
				}
			}
			unresolved_value = new_value
			value = resolvePointer(new_value).Interface()

		default:
			// TODO: Not allowed, return empty string. Maybe return an error in a future strict mode.
			fmt.Printf("Specifier '%v' not possible in accessing '%v' (of type %T).\n", specifier, value, value)
			return "", nil
		}
	}

	return value, nil
}

func newExpr(in *string) (*expr, error) {
	e := &expr{
		raw:     strings.TrimSpace(*in),
		filters: make([]exprFilterFunc, 0, 5),
	}

	err := e.parse()
	if err != nil {
		return nil, err
	}

	return e, nil
}

func convertTypeString(in string) (interface{}, error) {
	if len(in) == 0 {
		panic("This should never happen (len(in) == 0). Please report this bug.")
	}

	switch {
	case strings.HasPrefix(in, "\""):
		// Is string
		if !strings.HasSuffix(in, "\"") {
			return nil, errors.New(fmt.Sprintf("String not closed: '%s'", in))
		}
		if len(in) <= 1 {
			return nil, errors.New(fmt.Sprintf("String ('%s') malformed.", in))
		}
		return in[1 : len(in)-1], nil
	case in == "true" || in == "false":
		// Is bool
		b, err := strconv.ParseBool(in)
		if err != nil {
			return nil, err
		}
		return b, nil
	case in[0] >= '0' && in[0] <= '9':
		if strings.Contains(in, ".") {
			// Assuming float
			f, err := strconv.ParseFloat(in, 64)
			if err != nil {
				return nil, errors.New(fmt.Sprintf("Float is not valid: '%s' (%s)", in, err.Error()))
			}
			return f, nil

		} else {
			// Assuming int
			i, err := strconv.Atoi(in)
			if err != nil {
				return nil, errors.New(fmt.Sprintf("Integer is not valid: '%s' (%s)", in, err.Error()))
			}
			return i, nil
		}
	default:
		// Record the identifier for later lookup in the execution context
		// Only A-Za-z0-9_ is allowed
		if !exprIdentChecker.MatchString(in) {
			return nil, errors.New(fmt.Sprintf("Identifier ('%s') must only contain A-Za-z0-9_", in))
		}

		return exprIdent(in), nil
	}
	panic("not reachable")
}

func (e *expr) parse() error {
	// The raw string might contain: name|capitalize|format:"%s is cool! :)"

	// First check if we should negate the expression

	if strings.HasPrefix(e.raw, "!") {
		e.negate = true
		e.raw = e.raw[1:]
	}

	// Split the string into its parts
	parts := strings.Split(e.raw, "|")
	if len(parts) == 0 {
		return errors.New("Expression does not contain any data")
	}

	// Get root's type
	root := strings.TrimSpace(parts[0])

	if len(root) <= 0 {
		return errors.New("Identifier is an empty string")
	}

	// Check if identifier has arguments
	if !strings.HasPrefix(root, "\"") && strings.Contains(root, ":") {
		// Has args
		_args := strings.SplitN(root, ":", 2)
		root = _args[0]

		_split_args := *splitArgs(&_args[1], ",")

		args := make([]reflect.Value, 0, len(_split_args))

		// parse arguments
		for _, arg := range _split_args {
			_arg, err := convertTypeString(arg)
			if err != nil {
				return err
			}
			args = append(args, reflect.ValueOf(_arg))
		}
		e.root_args = args
	}

	id, err := convertTypeString(root)
	if err != nil {
		return err
	}
	e.root = id

	// Determine all filter functions and their arguments
	for _, part := range parts[1:] {
		var filtername string
		var args []interface{}

		part = strings.TrimSpace(part)

		if strings.Contains(part, ":") {
			// split filtername and args
			_args := strings.SplitN(part, ":", 2)
			filtername = _args[0]
			_split_args := *splitArgs(&_args[1], ",")

			// prepare args
			args = make([]interface{}, 0, len(_split_args))

			// parse arguments
			for _, arg := range _split_args {
				_arg, err := convertTypeString(arg)
				if err != nil {
					return err
				}
				args = append(args, _arg)
			}
		} else {
			// no args
			filtername = part
		}

		filterfn, has := Filters[filtername]
		if !has {
			return errors.New(fmt.Sprintf("Filter '%s' not found", filtername))
		}

		eff := exprFilterFunc{
			name: filtername,
			fn:   filterfn,
			args: args,
		}
		e.filters = append(e.filters, eff)
	}

	return nil
}

func (e *expr) String() string {
	return fmt.Sprintf("<expr root(%T)='%v' filters=%v>", e.root, e.root, e.filters)
}

func (e *expr) evalValue(ctx *Context) (interface{}, error) {
	// Check ctx for nil

	// Execute expression
	var value interface{} = e.root

	// If value is ident, look it up in context
	if name, is_ident := value.(exprIdent); is_ident {
		content, err := resolveIdent(name, ctx)
		if err != nil {
			return nil, err
		}

		// resolveIdent only returns a reflect.Value if there is a method to call
		if method, is_method := content.(reflect.Value); is_method {
			// Check whether the function gets all its required arguments, if not, set value to 
			// an empty string (TODO: in strict mode raise an error)

			mt := content.(reflect.Value).Type()
			if len(e.root_args) != mt.NumIn() {
				// Wrong argument count
				// TODO: Return an error in strict mode
				value = ""
			} else {
				// First see if we have to resolve some of the args
				for idx, arg := range e.root_args {
					// Example: {{ MsgTo:User,Msg }} with "User" and "Msg" from Context
					if ident, is_ident := arg.Interface().(exprIdent); is_ident {
						resolved_ident, err := resolveIdent(ident, ctx)
						if err != nil {
							return nil, err
						}
						e.root_args[idx] = reflect.ValueOf(resolved_ident)
					}
				}

				// TODO: Use .In() to see if the given arg types fit in.
				// TODO: Return an error in strict mode

				results := method.Call(e.root_args)
				if len(results) > 1 {
					return nil, errors.New(fmt.Sprintf("Method '%s' returns more than one value, this does not work.", string(name)))
				}
				if len(results) == 0 {
					return "", nil
				}
				if !results[0].CanInterface() {
					return "", nil
				}

				value = results[0].Interface()
				//fmt.Printf("result = %v\n", value)
			}
		} else {
			value = content
		}
	}

	var err error
	chainCtx := newFilterChainContext()
	for _, filter := range e.filters {
		// If there is no filter function, it only wants to be recorded in the chain-context.
		// For example, "safe" checks whether there is already an "unsafe"-filter (or the safe-filter itself already) applied. 
		if filter.fn != nil {
			// Prepare arguments and see if we have one we should resolve from Context
			for i := 0; i < len(filter.args); i++ {
				if ident, is_ident := filter.args[i].(exprIdent); is_ident {
					// Is ident, resolve it!
					resolved_ident, err := resolveIdent(ident, ctx)
					if err != nil {
						return nil, err
					}
					filter.args[i] = resolved_ident
				}
			}

			value, err = filter.fn(value, filter.args, chainCtx)
			if err != nil {
				return nil, errors.New(fmt.Sprintf("Filter '%s' failed: %s", filter.name, err.Error()))
			}
		}
		chainCtx.visitFilter(filter.name)
	}

	// Check for negation
	if e.negate {
		// Check whether it's a bool
		switch val := value.(type) {
		case bool:
			return !val, nil
		default:
			fmt.Printf("%v (type %T)\n", value, value)
			// If negation of a string, int or something, check whether they equal
			// their default value. Default behaviour is: empty type evaluates to false (since
			// this is a negation it must evaluating to true) 
			value = reflect.Zero(reflect.TypeOf(value)).Interface() == value

			// TODO: Not needed anymore?
			//return nil, errors.New(fmt.Sprintf("Cannot negate '%v' of type %T (maybe you want to add the unsafe-filter; filter history: %v).", value, value, chainCtx.applied_filters))
		}
	}

	return value, nil
}

func (e *expr) evalString(ctx *Context) (*string, error) {
	out, err := e.evalValue(ctx)
	if err != nil {
		return nil, err
	}
	outstr := fmt.Sprintf("%v", out)
	return &outstr, nil
}

func (e *expr) addFilter(name string) (bool, error) {
	filterfn, has := Filters[name]
	if !has {
		return false, errors.New(fmt.Sprintf("Filter '%s' not found", name))
	}

	eff := exprFilterFunc{
		name: name,
		fn:   filterfn,
	}
	e.filters = append(e.filters, eff)

	return true, nil
}
