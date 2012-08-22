package pongo

import (
	"strings"
)

func splitArgs(in *string, sep string) *[]string {
	if in == nil {
		panic("Implementation error; parseArgs got a nil string as input. Please report this issue.")
	}
	if len(sep) != 1 {
		panic("Separator must be exactly one char (string of length 1).")
	}

	res := make([]string, 0, strings.Count(*in, sep)+1) // approx count(sep)+1 args

	escaped := false
	in_string := false
	pos := 0
	buf := *in
	argbuf := ""
	pc := ""

	for pos < len(buf) {
		c := buf[pos : pos+1]
		if pos > 0 {
			pc = buf[pos-1 : pos]
		}

		// TODO: Handle string escape correctly (e. g. "this is \"nice\""), still too lazy to do
		if pc == "\\" {
			escaped = true
		} else {
			escaped = false
		}

		if c == "\"" && !escaped {
			if in_string {
				// String end
				in_string = false

				// We go a string, now add it to res
				argbuf += buf[:pos+1]
				buf = buf[pos+1:]
				pos = 0
			} else {
				// String found
				in_string = true
				pos++
			}
			continue
		}

		if in_string {
			pos++
			continue
		}

		if c == sep {
			// seperator found, add new arg
			res = append(res, argbuf)
			argbuf = ""
			buf = buf[pos+1:]
			pos = 0
			continue
		}

		argbuf += c
		pos++
	}

	// Is there a last argument?
	if len(argbuf) > 0 {
		res = append(res, argbuf)
	}

	return &res
}
