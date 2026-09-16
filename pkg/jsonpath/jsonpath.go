// Package jsonpath builds gjson/sjson paths from object keys and array indices.
//
// Kubernetes keys routinely contain characters that are path syntax: "kubernetes.io/os"
// holds a dot, and annotation or CR keys can hold '*', '?', '#' or '|'. A path built by
// joining raw keys addresses the wrong field, or is rejected by sjson.
package jsonpath

import (
	"strconv"
	"strings"
)

var escaper = strings.NewReplacer(`\`, `\\`, `.`, `\.`, `*`, `\*`, `?`, `\?`, `|`, `\|`, `#`, `\#`, `@`, `\@`, `!`, `\!`, `=`, `\=`, `<`, `\<`, `>`, `\>`, `%`, `\%`)

// Escape makes an object key usable as one path component.
func Escape(key string) string {
	return escaper.Replace(key)
}

// Join builds a path from already-escaped components.
func Join(parts ...string) string {
	return strings.Join(parts, ".")
}

// Index is the path component for an array element.
func Index(i int) string {
	return strconv.Itoa(i)
}
