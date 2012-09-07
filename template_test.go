package pongo

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	tpl    string  // The template to execute
	output string  // Expected output
	ctx    Context // Context for execution (can be nil) 
	err    string  // Expected error-message (part of it); if it contains "FUTURE" the test will be omitted.
}

var standard_tests = []test{
	// Plain text
	{"      ", "      ", nil, ""},
	{"Hallo !§$%&/()?==??&&", "Hallo !§$%&/()?==??&&", nil, ""},
	{"<script>if (true) { alert('yop'); }</script>", "<script>if (true) { alert('yop'); }</script>", nil, ""}, // See issue #1
	{`... Line 1
	... Line 2
	... Line 3
{% ... %}`, "", nil, "Line 4, Column 8"}, // Line/col tests
	{`... Line 1
	... Line 2
	... Line 3
	{{ }}`, "", nil, "Line 4, Column 5"}, // Line/col tests, with tab as one char

	// Comments
	{"{# This is a simple comment #}", "", nil, ""},
	{`{# This is a simple multi-line
	...
	comment
	
#}`, "", nil, ""},

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
	{"{{ name }}", "Pongo", Context{"name": "Pongo"}, ""},
	{"{{ name.3 }}", "g", Context{"name": "Pongo"}, ""},
	{"{{ name.three }}", "g", Context{"name": "Pongo", "three": 3}, ""},
	{"{{ name.three.0 }}", "g", Context{"name": "Pongo", "three": 3}, ""},

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

	// Time samples (no need for a date-filter, because you can simply call time's Format method from Pongo)
	{"{{ mydate.Format:\"02.01.2006 15:04:05\" }}", "18.08.2012 10:49:12", Context{"mydate": time.Date(2012, time.August, 18, 10, 49, 12, 0, time.Now().Location())}, ""},
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
	{"{{ \"<script>...</script>\"|safe|safe|safe }}", "&lt;script&gt;...&lt;/script&gt;", nil, ""}, // explicit multiple safes

	// Context-sensitive safety
	{"<a title='{{ \"Yep: This is <strong>cool</strong>!\" }}'>", "", nil, "FUTURE"},
	{"<a href='/{{ \"Yep: This is <strong>cool</strong>!\" }}'>", "", nil, "FUTURE"},
	{"<a href='?arg={{ \"Yep: This is <strong>cool</strong>!\" }}'>", "", nil, "FUTURE"},
	{"<a href='?{{ \"Yep: This is <strong>cool</strong>!\" }}'>", "", nil, "FUTURE"},
	{"<a href='testfn({{ \"Yep: This is <strong>cool</strong>!\" }});'>", "", nil, "FUTURE"},
	{"<a href='testfn({{ x }});'>", "", Context{"x": "Oh yeah, a string."}, "FUTURE"},
	{"<a href='testfn({{ 123591 }});'>", "", nil, "FUTURE"},
	{"<script>var foo = '{{ \"Yep: This is <strong>cool</strong>!\" }}';</script>", "", nil, "FUTURE"},
	{"<a href='{{ \"Yep: This is <strong>cool</strong>!\" }}'>", "", nil, "FUTURE"},
	{"<a href='{{ \"Yep: This is <strong>cool</strong>!\"|unsafe }}'>", "<a href='Yep: This is <strong>cool</strong>'>", nil, "FUTURE"},

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
	{"{% extends foobar %}  This doesn't show up {% block name %}Florian{% endblock %}", "Hello Florian!", Context{"foobar": "base"}, ""},
	{"{% extends foobar %}  This doesn't show up {% block name %}Florian{% endblock %}", "Hello Florian!", nil, "Please provide a propper template filename"},
	{"{% extends \"base\" %}  This doesn't show up", "Hello Josh!", nil, ""},
	{"{% extends \"base2\" %}  This doesn't show up {% block name %}Florian{% endblock %}", "", nil, "Could not find the template"},

	// Static extend (template will be pre-cached at startup and not dynamically rendered)
	// This improves speed significantly
	{"{% extends static \"base\" %}  This doesn't show up {% block name %}Florian{% endblock %}", "Hello Florian!", nil, ""},
	{"{% extends static foobar %}  This doesn't show up {% block name %}Florian{% endblock %}", "Hello Florian!", nil, "Please provide a propper template filename"},
	{"{% extends static \"base\" %}  This doesn't show up", "Hello Josh!", nil, ""},
	{"{% extends static \"base2\" %}  This doesn't show up {% block name %}Florian{% endblock %}", "", nil, "Could not find the template"},

	// Include
	{"{% include \"greetings\" %} How are you today?", "Hello Flo! How are you today?", Context{"name": "flo"}, ""},
	{"{% include tpl_name %} How are you today?", "Hello Flo! How are you today?", Context{"name": "flo", "tpl_name": "greetings"}, ""},
	{"{% include \"foobar\" %} This and that", "", nil, "Could not find the template"},
	{"{% include \"greetings_with_errors\" %} This and that", "", nil, "[Parsing error: greetings_with_errors] [Line 1, Column 27] Filter 'notexistent' not found"},
	
	// Static include (see comments for static extend above)
	{"{% include static \"greetings\" %} How are you today?", "Hello Flo! How are you today?", Context{"name": "flo"}, ""},
	{"{% include static tpl_name %} How are you today?", "Hello Flo! How are you today?", Context{"name": "flo", "tpl_name": "greetings"}, "Please provide a propper template filename"},
	{"{% include static tpl_name %} How are you today?", "Hello Flo! How are you today?", nil, "Please provide a propper template filename"},
	{"{% include static \"foobar\" %} This and that", "", nil, "Could not find the template"},
	{"{% include static \"greetings_with_errors\" %} This and that", "", nil, "[Parsing error: greetings_with_errors] [Line 1, Column 27] Filter 'notexistent' not found"},

	// Custom tag.. 
	// TODO
}

var string_tests = map[string][]test{
	"standard": standard_tests,
	"filter":   filter_tests,
	"tags":     tags_tests,
}

var file_tests = []test{
	// General
	{"template_examples/index1.html", "", Context{"basename": "generic/base-notexistent.html"}, "Could not find the template"},
	{"template_examples/index1.html", "<html><head><title>Myindex</title></head><body></body></html>", Context{"basename": "generic/base1.html"}, ""},
	{"template_examples/index1.html", "<html><head><title>Myindex</title></head><body></body></html>", nil, "Please provide a propper template filename"},

	// Static template caching
	{"template_examples/index2.html", "<html><head><title>Myindex</title></head><body></body></html>", nil, ""},
	{"template_examples/index3.html", "", nil, "Could not find the template"},
}

var base1 = "Hello {% block name %}Josh{% endblock %}!"
var greetings1 = "Hello {{ name|capitalize }}!"
var greetings_with_errors = "Hello {{ name|notexistent }}!"

func getTemplateCallback(name *string) (*string, error) {
	switch *name {
	case "base":
		return &base1, nil
	case "greetings":
		return &greetings1, nil
	case "greetings_with_errors":
		return &greetings_with_errors, nil
	default:
		return nil, errors.New("Could not find the template")
	}
	panic("unreachable")
}

func execTpl(t *test) (*string, error) {
	tpl, err := FromString("gotest", &t.tpl, getTemplateCallback)
	if err != nil {
		return nil, err
	}
	if t.ctx != nil {
		return tpl.Execute(&t.ctx)
	}
	return tpl.Execute(nil)
}

/*func BenchmarkTemplate(b *testing.B) {
	for i := 0; i < 1000; i++ {
		execTpl(`{{ name }}`, nil)
	}
}*/

func TestFromString(t *testing.T) {
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

	future_omitted := 0

	for name, testsuite := range string_tests {
		for _, test := range testsuite {
			if test.err == "FUTURE" {
				future_omitted++
				continue
			}

			out, err := execTpl(&test)
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

	if future_omitted > 0 {
		t.Logf("%d tests omitted, because they are flagged as FUTURE.", future_omitted)
	}
}

func TestFromFile(t *testing.T) {
	for _, test := range file_tests {
		name := test.tpl

		if !filepath.IsAbs(name) {
			abs_name, err := filepath.Abs(name)
			if err != nil {
				t.Fatalf(err.Error())
			}
			name = abs_name
		}

		tpl, err := FromFile(name, nil)
		if err != nil {
			if test.err != "" {
				if strings.Contains(strings.ToLower(err.Error()), strings.ToLower(test.err)) {
					// Err found which is expected
					continue
				}
				t.Errorf("File-Test '%s' FAILED (was expecting '%s' in error msg): %v", test, test.err, err)
				continue
			}
			t.Errorf("File-Test '%s' FAILED: %v", test, err)
			continue
		}
		var out *string
		if test.ctx != nil {
			out, err = tpl.Execute(&test.ctx)
		} else {
			out, err = tpl.Execute(nil)
		}
		if err != nil {
			if test.err != "" {
				if strings.Contains(strings.ToLower(err.Error()), strings.ToLower(test.err)) {
					// Err found which is expected
					continue
				}
				t.Errorf("File-Test '%s' FAILED (was expecting '%s' in error msg): %v", test, test.err, err)
				continue
			}
			t.Errorf("File-Test '%s' FAILED: %v", test, err)
			continue
		}
		if test.err != "" {
			t.Errorf("File-Test '%s' SUCCEEDED, but FAIL ('%s' in error msg) was EXPECTED; got output: '%s'", test, test.err, *out)
			continue
		}
		if *out != test.output {
			t.Errorf("File-Test '%s' FAILED; got='%s' should='%s'", test, *out, test.output)
			continue
		}
	}
}

// TODO:
// - Add Must() tests
// - Add thread-safety tests.

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
