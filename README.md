GoTemplate is a template engine which implements a [Django-template](https://docs.djangoproject.com/en/dev/topics/templates/)-like syntax.

Please have a look at the test (`template_test.go`) for examples.

# A tiny example

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

# Documentation

See GoPkgDoc for a list of implemented filters/tags and how to use the simple API:

[http://go.pkgdoc.org/github.com/flosch/GoTemplate](http://go.pkgdoc.org/github.com/flosch/GoTemplate)

You can simply add your own filters/tags. See the template_test.go for an example implementation.

# Status

GoTemplate is still in beta and has a very few known bugs (this is why the tests fail).

# License

GoTemplate is licensed under the MIT-license (see LICENSE file for more).
