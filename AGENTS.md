### Basic Go Stuff
* keep line length to around 80 characters; prefer vertical to horizontal
* one line per comma-delimited struct or array field.
* follow the user's requirements carefully & to the letter
* never write comments
* Use the standard library's net/http package for API development
* Use structured logging (slog) when possible
* use simple fmt.Errorf() error wrapping, include only information in the format string that the CALLER would not have
* If unsure about a best practice or implementation detail, say so instead of guessing.
* Prefer small helper functions to repeated code
* Use named functions for goroutines unless the goroutine only spans a couple lines
* "defer" unlocks of locks when possible rather than bracketing critical sections.
* don't create "else" arms on error checking conditions; keep code straight-line
* be familiar with RESTful API design principles, best practices, and Go idioms
* use fmt.Fprintf in preference to Write([]byte(stringValue))
* use StringBuilder or something like it in preference to append/join
* when possible/sensible, accumulate repetitive errors with errors.Join so we can do a single error check
* don't create new .go files until there's enough stuff for the file to hold multiple funcs, unless I say otherwise.

### Go Web Apps

* If we need to inject state into handlers, define a server struct to hold the state, don't write middleware for each handler.
* Do create a .go file for each HTTP handler if the handler does anything significant.
* NEVER define an inline handler in main or where HandlerFunc() is called. I will lose my shit.
