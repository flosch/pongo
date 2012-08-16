package template

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

type Person struct {
	Name        string
	Age         int
	Friends     []*Person
	Accounts    map[string]float64
	notexported int
}

type Person2 struct {
	Name        string
	Age         int
	Friends     []Person
	notexported int
}

func (p *Person) SayHello() string {
	return "Hello Flo!"
}

func (p *Person) SayHelloTo(name1, name2 string) string {
	return fmt.Sprintf("Hello to %s and %s from Flo!", name1, name2)
}

var (
	person = Person{
		Name: "Florian",
		Age:  40,
		Friends: []*Person{
			&Person{Name: "Georg", Age: 51},
			&Person{Name: "Mike", Age: 25},
			&Person{Name: "Philipp", Age: 19},
		},
		Accounts: map[string]float64{
			"default": 1234.56,
		},
	}
	person2 = Person2{
		Name: "Florian",
		Age:  40,
		Friends: []Person{
			Person{Name: "Georg", Age: 51},
			Person{Name: "Mike", Age: 25},
			Person{Name: "Philipp", Age: 19},
		},
	}
)

type test struct {
	tpl    string
	output string
	ctx    Context
	err    string
}

var standard_tests = []test{
	// Plain text
	{"      ", "      ", nil, ""},
	{"Hallo !§$%&/()?==??&&", "Hallo !§$%&/()?==??&&", nil, ""},
	{`... Line 1
	... Line 2
	... Line 3
{% ... %}`, "", nil, "Line 4, Column 8"}, // Line/col tests
	{`... Line 1
	... Line 2
	... Line 3
	{{ }}`, "", nil, "Line 4, Column 5"}, // Line/col tests, with tab as one char

	// Trivial errors
	{"", "", nil, "Template has no content"},
	{"{{ }}", "", nil, "Identifier is an empty string"},

	// Strings
	{"<{{\"\"}}>", "<>", nil, ""},
	{"{{ \"Hallo\" }}", "Hallo", nil, ""},
	{"{{ \"Hallo\".4 }}", "o", nil, ""},

	// Int
	{"{{ 5 }}", "5", nil, ""},
	{"{{ 5.499999999999999 }}", "5.499999999999999", nil, ""},
	{"{{ 5.499999999999999. }}", "", nil, "Float is not valid"},

	// Bool (and negation)
	{"{{ true }}", "true", nil, ""},
	{"{{ false }}", "false", nil, ""},
	{"{{ !5|unsafe }}", "", nil, "Cannot negate '5' of type int"},
	{"{{ !true }}", "", nil, "maybe you want to add the unsafe-filter"},
	{"{{ !false }}", "", nil, "maybe you want to add the unsafe-filter"},
	{"{{ !true|unsafe }}", "false", nil, ""},
	{"{{ !false|unsafe }}", "true", nil, ""},

	// Simple variables
	{"{{ foo }}", "", nil, ""},
	{"{{ foo.bar }}", "", nil, ""},
	{"{{ name }}", "GoTemplate", Context{"name": "GoTemplate"}, ""},
	{"{{ name.5 }}", "p", Context{"name": "GoTemplate"}, ""},
	{"{{ name.five }}", "p", Context{"name": "GoTemplate", "five": 5}, ""},
	{"{{ name.five.0 }}", "p", Context{"name": "GoTemplate", "five": 5}, ""},

	// Context item access
	{"{{ person.Name }}", "Florian", Context{"person": &person}, ""},
	{"{{ person.Name.1 }}", "l", Context{"person": &person}, ""},
	{"{{ person.Name.two }}", "o", Context{"person": &person, "two": 2}, ""},
	{"{{ person.Name.17 }}", "", Context{"person": &person}, ""},
	{"{{ person.Age }}", "40", Context{"person": &person}, ""},
	{"{{ person.alter }}", "40", Context{"person": &person, "alter": "Age"}, ""},
	{"{{ person.Friends }}", "[{Georg 51 [] map[] 0} {Mike 25 [] map[] 0} {Philipp 19 [] map[] 0}]", Context{"person": person2}, ""},
	{"{{ person.Friends.0 }}", "{Georg 51 [] map[] 0}", Context{"person": &person}, ""},
	{"{{ person.Friends.0.Name }}", "Georg", Context{"person": &person}, ""},
	{"{{ person.Friends.0.Friends }}", "[]", Context{"person": &person}, ""},
	{"{{ person.Friends.0.Friends.0 }}", "", Context{"person": &person}, ""},
	{"{{ person.Friends.two }}", "{Philipp 19 [] map[] 0}", Context{"person": &person, "two": 2}, ""},
	{"{{ person.Friends.99 }}", "", Context{"person": &person}, ""},
	{"{{ person.Accounts.default }}", "1234.56", Context{"person": &person}, ""},
	{"{{ person.Accounts.alternative }}", "1234.56", Context{"person": &person, "alternative": "default"}, ""},
	{"{{ person.Accounts.notexistent }}", "", Context{"person": &person}, ""},
	{"{{ person.Accounts.notexistent|default:0.0 }}", "0", Context{"person": &person}, ""}, // w/ default
	{"{{ person.notexported }}", "", Context{"person": &person}, ""},
	{"{{ person.foobar }}", "", Context{"person": &person}, ""},
	{"{{ person.Foobar|default:\"no foobar\" }}", "no foobar", Context{"person": &person}, ""},

	// Method calls	
	{"{{ person.SayHello }}", "Hello Flo!", Context{"person": &person}, ""},                                                                                   // lazy call (w/ pointer)
	{"{{ person.SayHello }}", "", Context{"person": person}, ""},                                                                                              // lazy call (w/o pointer)
	{"{{ person.SayHello.0 }}", "H", Context{"person": &person}, ""},                                                                                          // direct call (w/ pointer)
	{"{{ person.SayHello.0 }}", "", Context{"person": person}, ""},                                                                                            // direct call (w/o pointer)
	{"{{ person.SayHelloTo:\"Cowboy, Mike\",\"Cowboy, Thorsten\" }}", "Hello to Cowboy, Mike and Cowboy, Thorsten from Flo!", Context{"person": &person}, ""}, // call w/ args (w/ pointer) 
	{"{{ person.SayHelloTo:\"Cowboy, Mike\",\"Cowboy, Thorsten\" }}", "", Context{"person": person}, ""},                                                      // call w/ args (w/o pointer)
	{"{{ person.SayHelloTo:5,\"Cowboy, Thorsten\" }}", "", Context{"person": person}, ""},                                                                     // call w/ args (w/o pointer) (wrong arg type)
}

var filter_tests = []test{
	// General
	{"{{    \"florian\"    |           capitalize        |safe    }}", "Florian", nil, ""}, // spaces between filters

	// Trim
	{"{{\"      Florian       \"|trim}}", "Florian", nil, ""},
	{"{{ 5|trim }}", "Florian", nil, "is not of type string"},

	// Lower + upper
	{"{{ name|lower }}", "florian", Context{"name": "FlOrIaN"}, ""},
	{"{{ name|upper }}", "FLORIAN", Context{"name": "FlOrIaN"}, ""},
	{"{{ \"flOrIaN\"|lower }}", "florian", nil, ""},
	{"{{ \"flOrIaN\"|upper }}", "FLORIAN", nil, ""},
	{"{{ 5|lower }}", "", nil, "not of type string"},
	{"{{ true|upper }}", "", nil, "not of type string"},

	// Trim
	{"{{ \"      florian	\"|trim }}", "florian", nil, ""},
	{"{{ name|trim }}", "Florian", Context{"name": "		   Florian  	  	"}, ""},
	{"{{ 2|trim }}", "", nil, "not of type string"},

	// Safe + Unsafe
	{"{{ \"<script>...</script>\" }}", "&lt;script&gt;...&lt;/script&gt;", nil, ""},                // auto-safe
	{"{{ \"<script>...</script>\"|unsafe }}", "<script>...</script>", nil, ""},                     // unsafe
	{"{{ \"<script>...</script>\"|safe|safe|safe }}", "&lt;script&gt;...&lt;/script&gt;", nil, ""}, // auto-safe, // explicit multiple safes

	// Default
	{"{{ \"\"|default:\"yes\" }}", "yes", nil, ""},
	{"{{ false|default:\"yes\" }}", "yes", nil, ""},
	{"{{ true|default:\"no\" }}", "true", nil, ""},
	{"{{ notexistent|default:\"yes\" }}", "yes", nil, ""},

	// Capitalize
	{"{{ name|capitalize }}", "Florian", Context{"name": "florian"}, ""},
	{"{{ \"florian\"|capitalize }}", "Florian", nil, ""},
	{"{{ 5|capitalize }}", "", nil, "not of type string"},

	// Length
	{"{{ name|length }}", "7", Context{"name": "Florian"}, ""},
	{"{{ \"florian\"|length }}", "7", nil, ""},
	{"{{ 5|length }}", "", nil, "Cannot determine length from type int"},

	// Join
	{"{{ names|join:\", \" }}", "Florian, Georg, Timm", Context{"names": []string{"Florian", "Georg", "Timm"}}, ""},
	{"{{ 5|join:\"-\" }}", "", nil, "Cannot join variable of type int"},

	// Striptags
	{"{{ \"<strong><em>Hi Florian!</em></strong>\"|striptags:\"strong\" }}", "&lt;em&gt;Hi Florian!&lt;/em&gt;", nil, ""},
	{"{{ \"<strong><em>Hi Florian!</em></strong>\"|striptags:\"strong\"|unsafe }}", "<em>Hi Florian!</em>", nil, ""},
	{"{{ \"<strong><em>Hi Florian!</em></strong>\"|striptags:\"strong,em\" }}", "Hi Florian!", nil, ""},
	{"{{ \"<strong><em>Hi Florian!</em></strong><img /></img>\"|striptags }}", "Hi Florian!", nil, ""}, // remove all tags
	{"{{ 5|striptags:\"x\" }}", "", nil, "not of type string"},
	{"{{ \"\"|striptags:\"x\",123 }}", "", nil, "Please provide a comma-seperated string with tags (or no string to remove all tags)."},

	// Custom 'add' filter (see the TestSuites(*testing.T) function)
	{"{{ 5|add:7 }}", "12", nil, ""},
	{"{{ 5|add:7,Seven }}", "19", Context{"Seven": 7}, ""},
	{"{{ 5|add:7,10 }}", "22", nil, ""},
	{"{{ 5|add:7,10,25 }}", "47", nil, ""},
	{"{{ 5|add:10,\"test\" }}", "", nil, "No int: test"},
}

var tags_tests = []test{
	// General
	{"{% %}", "", nil, "Tag '' does not exist"},
	{"{% if test %}", "", nil, "No end-node"},

	// If-tag with...

	// ... bools
	{"{%if%}", "", nil, "If-argument is empty"},
	{"{%if true%}Yes{% else %}No{%endif%}", "Yes", nil, ""},
	{"{% if !true %}Yes{% else %}No{%endif%}", "No", nil, ""},
	{"{% if false %}Yes{% else %}No{%endif%}", "No", nil, ""},
	{"{% if !false %}Yes{% else %}No{%endif%}", "Yes", nil, ""},
	{"{% if true && true && true && true && !false && !false %}Yes{% else %}No{%endif%}", "Yes", nil, ""},
	{"{% if !true || !true || !true || true && false %}Yes{% else %}No{%endif%}", "No", nil, ""},
	{"{% if !true || !true || !true || true && true %}Yes{% else %}No{%endif%}", "Yes", nil, ""},
	{"{% if false || false || true || false %}Yes{% else %}No{%endif%}", "Yes", nil, ""},
	{"{% if false || true %}Yes{% else %}No{%endif%}", "Yes", nil, ""},

	// ... strings
	{"{% if \"\" %}Yes{% else %}No{%endif%}", "No", nil, ""}, // an empty string evaluates to false
	{"{% if \"with content\" %}Yes{% else %}No{%endif%}", "Yes", nil, ""},

	// ... ints
	{"{% if 0 %}Yes{% else %}No{%endif%}", "No", nil, ""}, // 0 evaluates to false
	{"{% if zero %}Yes{% else %}No{%endif%}", "No", Context{"zero": 0}, ""},
	{"{% if 1 %}Yes{% else %}No{%endif%}", "Yes", nil, ""},
	{"{% if 919592 %}Yes{% else %}No{%endif%}", "Yes", nil, ""},

	// ... floats
	{"{% if 0.0 %}Yes{% else %}No{%endif%}", "No", nil, ""}, // 0.0 evaluates to false
	{"{% if zero %}Yes{% else %}No{%endif%}", "No", Context{"zero": 0}, ""},
	{"{% if 1 %}Yes{% else %}No{%endif%}", "Yes", nil, ""},
	{"{% if 919592 %}Yes{% else %}No{%endif%}", "Yes", nil, ""},

	// nested if's
	{"{% if person.Age > 0 %}{% if person.Age > 50 %}yes{% if person.Age > 60 %}no{% else %}yes{% endif %}{% else %}no2{% endif %}{% else %}no1{% endif %}", "no2", Context{"person": person}, ""},
	{"{% if person.Age > 0 && person.Age >= 40 %}yes{%else%}no{% endif %}", "yes", Context{"person": person}, ""},
	{"{% if person.Age > 0 && person.Age >= 40 %}yes{%else%}no{% endif %}", "yes", Context{"person": &person}, ""},
	{"{% if person.Age > 0 && person.Age > 40 %}yes{%else%}no{% endif %}", "no", Context{"person": person}, ""},
	{"{% if person.Age > 0 && person.Age >= 40 && person.Age < 41 %}yes{%else%}no{% endif %}", "yes", Context{"person": person}, ""},

	{"{% if false %}{% if person.Age > 50 %}yes{% if person.Age > 60 %}no{% else %}yes{% endif %}{% else %}no2{% endif %}{% else %}no1{% endif %}", "no1", nil, ""},

	// misc
	{"{% if 5 && 10 %}Yes{%else%}No{%endif%}", "No", nil, ""}, // Non-bool expressions evaluating to false  
	{"{% if \"Flo==ri&&an\"|lower == \"flo==ri&&an\" %}yes{%else%}no{%endif%}", "yes", nil, ""},
	{"{% if name|lower == \"flo==ri&&an\" %}yes{%else%}no{%endif%}", "yes", Context{"name": "flo==ri&&an"}, ""},
	{"{% if name == \"flo==ri&&an\" %}yes{%else%}no{%endif%}", "yes", Context{"name": "flo==ri&&an"}, ""},

	// For
	{"{% for 6 %}{{ forloop.Counter }}{% endfor %}", "012345", nil, ""},
	{"{% for 6 %}{{ forcounter }}{% endfor %}", "012345", nil, ""},
	{"{% for 6 %}{{ forloop.Counter1 }}{% endfor %}", "123456", nil, ""},
	{"{% for 6 %}{{ forcounter1 }}{% endfor %}", "123456", nil, ""},
	{"{% for 6 %}{{ forloop.Max }}{% endfor %}", "555555", nil, ""},
	{"{% for 6 %}{{ forloop.Max1 }}{% endfor %}", "666666", nil, ""},
	{"{% for 6 %}{{ forloop.First }}{% endfor %}", "truefalsefalsefalsefalsefalse", nil, ""},
	{"{% for 6 %}{{ forloop.Last }}{% endfor %}", "falsefalsefalsefalsefalsetrue", nil, ""},
	{"{% for 0 %}Yes{% else %}No{% endfor %}", "No", nil, ""},                                                                                                    // else-block in for-loops
	{"{% for name|length %}Yes{% else %}No{% endfor %}", "No", nil, ""},                                                                                          // else-block in for-loops
	{"{% for name|length %}{{ name.forcounter }}{% endfor %}", "Florian", Context{"name": "Florian"}, ""},                                                        // strings in forloops
	{"{% for char in name %}{{ char }}{% endfor %}", "Florian", Context{"name": "Florian"}, ""},                                                                  // strings in forloops 
	{"{% for word in words %}{{ word|capitalize }}{% if !forloop.Last %} {%endif %}{% endfor %}", "Hi Florian", Context{"words": []string{"hi", "florian"}}, ""}, // slices in for-loops
	{"{% for word in words %}{{ word.Key }} means {{ word.Value }}{% endfor %}", "salut means hello", Context{"words": map[string]string{"salut": "hello"}}, ""}, // maps in for-loops
	{"{% for friend in person.Friends %}{{ friend.Name }}{% endfor %}", "Florian", Context{"person": Person{Friends: []*Person{&Person{Name: "Florian"}}}}, ""},  // slices with structs in for-loops

	// Nested forloops and use of forloop/forloops
	{"{% for 3 %}{{ forloop.Counter1 }}{%for 6%}{{ forloop.Counter1 }}{% endfor %}{% endfor %}", "112345621234563123456", nil, ""},                                                                                                                                                                                                                                                                                                                                  // addressing their respective for-loop-context
	{"{% for 3 %}{%for 6%}{{ forloops.0.Counter1 }}{{ forloops.1.Counter1 }}{% endfor %}{% endfor %}", "111213141516212223242526313233343536", nil, ""},                                                                                                                                                                                                                                                                                                             // using forloops (plural-s) to address the outer and the inner for-loop-context (2 nested loops)
	{"{% for 3 %}{%for 6%}{% for 4 %}{{ forloops.0.Counter1 }}{{ forloops.1.Counter1 }}{{forloops.2.Counter1 }} {% endfor %}{% endfor %}{% endfor %}", "111 112 113 114 121 122 123 124 131 132 133 134 141 142 143 144 151 152 153 154 161 162 163 164 211 212 213 214 221 222 223 224 231 232 233 234 241 242 243 244 251 252 253 254 261 262 263 264 311 312 313 314 321 322 323 324 331 332 333 334 341 342 343 344 351 352 353 354 361 362 363 364 ", nil, ""}, // using forloops (plural-s) to address the outer and the inner for-loop-context (3 nested loops)
	{"{% for word in words %}{% for char in word %}{{ forloops.0.Counter }}{{ forloops.1.Counter }}{{ char }}{% endfor %}{% endfor %}", "00H01e02l03l04o10F11l12o", Context{"words": []string{"Hello", "Flo"}}, ""},                                                                                                                                                                                                                                                 // using forloops
	{`{% trim %}{% for 0 %}
		{% for 10 %}
			{% for 100 %}
				{% for 1000 %}
					yes
				{% else %}
					else1000
				{% endfor %}
			{% else %}
				else100
			{% endfor %}
		{% else %}
			else10
		{% endfor %}
	{% else %}else0{% endfor %}{% endtrim %}`, "else0", nil, ""},
	{`{% trim %}{% for 3 %}{% for 0 %}{% for 100 %}{% for 1000 %}yes{% else %}else1000{% endfor %}{% else %}else100{% endfor %}{% else %}else10{% endfor %}{% else %}else0{% endfor %}{% endtrim %}`, "else10else10else10", nil, ""},
	{`{% remove %}{% for 1 %}
		{% for 2 %}
			{% for 3 %}
				{% for 0 %}
					yes	
				{% else %}
					else1000
				{% endfor %}
			{% else %}	
				else100
			{% endfor %}	
		{% else %}
			else10
		{% endfor %}
	{% else %}
		else0
	{% endfor %}{% endremove %}`, "else1000else1000else1000else1000else1000else1000", nil, ""},

	// Trim-tag
	{"{% trim %}	          hello     	 	{% endtrim %}", "hello", nil, ""},
	{"{% trim %}	  {% if true %}	          hello     	{% endif %}   	 	{% endtrim %}", "hello", nil, ""},
	{"{% trim %}	  {% if false %}	          hello     	{% endif %}   	 	{% endtrim %}", "", nil, ""},
	{"{% trim %}	  {% if false %}	          hello{% endtrim %}     	{% endif %}   	 	", "", nil, "No end-node (possible nodes: [endtrim]) found."},
	{"{% trim %}	  {% if true %}	          hello{% endtrim %}     	{% endif %}   	 	", "", nil, "Unhandled placeholder"},

	// Remove-tag
	{"{% remove \" \",\"\t\" %}	          hello     	 	{% endremove %}", "hello", nil, ""},
	{"{% remove \"hello\",\" \",\"\t\" %}	  {% if true %}	          hello     	{% endif %}   	 	{% endremove %}", "", nil, ""},
	{"{% remove \"hello\",\" \",\"\t\" %}	  {% if false %}	          hello     	{% endif %}   	 	{% endremove %}", "", nil, ""},
	{"{% remove %}	  {% if false %}	          hello    		{%else%}   yes 	{% endif %}   	 	{% endremove %}", "yes", nil, ""}, // remove without any argument defaults to empty spaces, tabs and new lines.

	// Block/Extends
	{"{% extends \"base\" %}  This doesn't show up {% block name %}Florian{% endblock %}", "Hello Florian!", nil, ""},

	// Custom tag.. 
	// TODO
}

var tests = map[string][]test{
	"standard": standard_tests,
	"filter":   filter_tests,
	"tags":     tags_tests,
}

var base1 = "Hello {% block name %}Josh{% endblock %}!"

func getTemplateCallback(name *string) (*string, error) {
	switch *name {
	case "base":
		return &base1, nil
	default:
		return nil, errors.New("Could not find the template")
	}
	panic("unreachable")
}

func execTpl(in string, ctx *Context) (*string, error) {
	tpl, err := FromString("gotest", &in, getTemplateCallback)
	if err != nil {
		return nil, err
	}
	return tpl.Execute(ctx)
}

/*func BenchmarkTemplate(b *testing.B) {
	for i := 0; i < 1000; i++ {
		execTpl(`{{ name }}`, nil)
	}
}*/

func TestSuites(t *testing.T) {
	// Provide custom filter
	Filters["add"] = func(value interface{}, args []interface{}, ctx *FilterChainContext) (interface{}, error) {
		i, is_int := value.(int)
		if !is_int {
			return nil, errors.New(fmt.Sprintf("No int: %v", value))
		}
		for _, arg := range args {
			a, is_int := arg.(int)
			if !is_int {
				return nil, errors.New(fmt.Sprintf("No int: %v", arg))
			}
			i += a
		}
		return i, nil
	}

	// Provide custom tag
	Tags["set"] = nil // TODO

	for name, testsuite := range tests {
		for _, test := range testsuite {
			out, err := execTpl(test.tpl, &test.ctx)
			if err != nil {
				if test.err != "" {
					if strings.Contains(strings.ToLower(err.Error()), strings.ToLower(test.err)) {
						// Err found which is expected
						continue
					}
					t.Errorf("[Suite: %s] Test '%s' FAILED (was expecting '%s' in error msg): %v", name, test.tpl, test.err, err)
					continue
				}
				t.Errorf("[Suite: %s] Test '%s' FAILED: %v", name, test.tpl, err)
				continue
			}
			if test.err != "" {
				t.Errorf("[Suite: %s] Test '%s' SUCCEEDED, but FAIL ('%s' in error msg) was EXPECTED; got output: '%s'", name, test.tpl, test.err, *out)
				continue
			}
			if *out != test.output {
				t.Errorf("[Suite: %s] Test '%s' FAILED; got='%s' should='%s'", name, test.tpl, *out, test.output)
				continue
			}
		}
	}
}

func ExampleParseArgs() {
	in := `15029582`
	r := splitArgs(&in, ",")
	for _, item := range *r {
		fmt.Printf("'%s'\n", item)
	}

	in = `"hello, florian!"`
	r = splitArgs(&in, ",")
	for _, item := range *r {
		fmt.Printf("'%s'\n", item)
	}

	in = `"hello, florian!",123,456,blahblah,foo,,"this is \"nice\", isn't it?","yeah it is, dude.",1`
	r = splitArgs(&in, ",")
	for _, item := range *r {
		fmt.Printf("'%s'\n", item)
	}

	// Output:
	// '15029582'
	// '"hello, florian!"'
	// '"hello, florian!"'
	// '123'
	// '456'
	// 'blahblah'
	// 'foo'
	// ''
	// '"this is \"nice\", isn't it?"'
	// '"yeah it is, dude."'
	// '1'

}
