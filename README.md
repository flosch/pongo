**I deactivated the issue tracker and wiki pages for pongo. pongo won't receive bugfixes anymore. Please consider using the successor [pongo2](https://github.com/flosch/pongo2) instead. You can find [more information and a migration tutorial](http://www.florian-schlachter.de/post/pongo2/) on my website.**

pongo is a well-tested template engine which implements a [Django-template](https://docs.djangoproject.com/en/dev/topics/templates/)-like syntax.

Please have a look at the test (`template_test.go`) for examples.

# A tiny example (template string)

	in := "Hello {{ name|capitalize }}!"
	tpl, err := pongo.FromString("mytemplatetest", &in, nil)
	if err != nil {
		panic(err)
	}
	out, err := tpl.Execute(&pongo.Context{"name": "florian"})
	if err != nil {
		panic(err)
	}
	fmt.Println(*out) // Output: Hello Florian!

# Example server-usage (template file)

	package main
	
	import (
		"github.com/flosch/pongo"
		"net/http"
	)
	
	var tplExample = pongo.Must(pongo.FromFile("example.html", nil))
	
	func examplePage(w http.ResponseWriter, r *http.Request) {
		err := tplExample.ExecuteRW(w, &pongo.Context{"query": r.FormValue("query")})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
	
	func main() {
		http.HandleFunc("/", examplePage)
		http.ListenAndServe(":8080", nil)
	}

# Documentation

See the wiki (work in progress) on GitHub for a documentation/reference:

[https://github.com/flosch/pongo/wiki](https://github.com/flosch/pongo/wiki).

While I'm working on the wiki content, GoPkgDoc shows a list of implemented filters/tags and an auto-generated documentation on how to use the simple API:

[http://go.pkgdoc.org/github.com/flosch/pongo](http://go.pkgdoc.org/github.com/flosch/pongo)

It is possible to add your own filters/tags. See the `template_test.go` for example implementations.

# Status

pongo is still in beta and has a very few known bugs (this is why the tests fail).

# License

pongo is licensed under the MIT-license (see LICENSE file for more).
