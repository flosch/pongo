Pongo is a template engine which implements a [Django-template](https://docs.djangoproject.com/en/dev/topics/templates/)-like syntax.

Please have a look at the test (`template_test.go`) for examples.

# A tiny example (template string)

	in := "Hello {{ name|capitalize }}!"
	tpl, err := template.FromString("mytemplatetest", &in, nil)
	if err != nil {
		panic(err)
	}
	out, err := tpl.Execute(&template.Context{"name": "florian"})
	if err != nil {
		panic(err)
	}
	fmt.Println(*out) // Output: Hello Florian!

# Example server-usage (template file)

	package main
	
	import (
		"github.com/flosch/Pongo"
		"net/http"
	)
	
	var tplExample = template.Must(template.FromFile("example.html", nil))
	
	func examplePage(w http.ResponseWriter, r *http.Request) {
		err := tplExample.ExecuteRW(w, nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
	
	func main() {
		http.HandleFunc("/", examplePage)
		http.ListenAndServe(":8080", nil)
	}

# Documentation

See GoPkgDoc for a list of implemented filters/tags and how to use the simple API:

[http://go.pkgdoc.org/github.com/flosch/Pongo](http://go.pkgdoc.org/github.com/flosch/Pongo)

You can simply add your own filters/tags. See the template_test.go for an example implementation.

# Status

Pongo is still in beta and has a very few known bugs (this is why the tests fail).

# License

Pongo is licensed under the MIT-license (see LICENSE file for more).
