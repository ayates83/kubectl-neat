package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf8"

	"github.com/ghodss/yaml"
	"github.com/tidwall/gjson"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
)

// FuzzNeatYAMLOrJSON checks that no input makes neat panic or silently discard a document,
// and that output it produces from a well-formed object is stable when neated again.
func FuzzNeatYAMLOrJSON(f *testing.F) {
	seeds, _ := filepath.Glob("../test/fixtures/*")
	for _, s := range seeds {
		if b, err := os.ReadFile(s); err == nil {
			f.Add(b, true)
			f.Add(b, false)
		}
	}
	f.Add([]byte("---\n# c\n---\napiVersion: v1\nkind: Service\nmetadata: {name: a}\nspec: {clusterIP: None}\n---\napiVersion: batch/v1\nkind: Job\nmetadata: {name: j}\nspec: {completions: 1, parallelism: 1}\n"), false)
	f.Fuzz(func(t *testing.T, in []byte, strip bool) {
		if !utf8.Valid(in) {
			return // the API server rejects it; JSON re-encoding would only change how it is escaped
		}
		if strip {
			userAnnotations, userLabels = []string{"*"}, []string{"app", "example.com/*"}
			defer func() { userAnnotations, userLabels = nil, nil }()
		}
		for _, format := range []string{"same", "json", "yaml"} {
			out, err := NeatYAMLOrJSON(in, format)
			if err != nil {
				continue
			}
			if isJSON(in) && gjson.ValidBytes(in) && gjson.ParseBytes(in).IsObject() && len(bytes.TrimSpace(out)) == 0 {
				t.Fatalf("a JSON object produced empty output (%s): %q", format, in)
			}
			again, err := NeatYAMLOrJSON(out, format)
			if err != nil {
				t.Fatalf("neat accepted its own %s output only once: %v\ninput: %q\noutput: %q", format, err, in, out)
			}
			if format != "same" && wellFormed(in) && wellFormed(out) && string(again) != string(out) {
				t.Fatalf("not idempotent (%s)\ninput: %q\nonce:  %q\ntwice: %q", format, in, out, again)
			}
		}
	})
}

// wellFormed reports whether every object key in a JSON or YAML document is non-empty and
// unique within its object, as the Kubernetes API requires. Paths cannot address the rest.
func wellFormed(doc []byte) bool {
	docs, err := splitYAMLDocuments(doc)
	if isJSON(doc) {
		docs, err = [][]byte{doc}, nil
	}
	if err != nil {
		return false
	}
	for _, d := range docs {
		j := d
		if !isJSON(d) {
			if j, err = yaml.YAMLToJSON(d); err != nil {
				return false
			}
		}
		if !uniqueKeys(gjson.ParseBytes(j)) {
			return false
		}
		// A field of the wrong type cannot come from the API server, and it stops defaulting.
		var pom metav1.PartialObjectMetadata
		if json.Unmarshal(j, &pom) == nil && scheme.Scheme.Recognizes(pom.GroupVersionKind()) {
			if _, _, err := scheme.Codecs.UniversalDeserializer().Decode(j, nil, nil); err != nil {
				return false
			}
		}
	}
	return true
}

func uniqueKeys(r gjson.Result) bool {
	ok := true
	seen := map[string]bool{}
	r.ForEach(func(k, v gjson.Result) bool {
		if r.IsObject() {
			if k.Str == "" || seen[k.Str] {
				ok = false
				return false
			}
			seen[k.Str] = true
		}
		if (v.IsObject() || v.IsArray()) && !uniqueKeys(v) {
			ok = false
			return false
		}
		return true
	})
	return ok
}
