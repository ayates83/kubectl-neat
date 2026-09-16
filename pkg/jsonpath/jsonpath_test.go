package jsonpath

import (
	"testing"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func TestEscapeAddressesExactKey(t *testing.T) {
	keys := []string{"kubernetes.io/os", "a*b", "a?b", "#000", "x|y", "@this", "!", "a=b", "<>", "50%", `back\slash`, "plain"}
	for _, k := range keys {
		doc := `{"spec":{}}`
		doc, err := sjson.Set(doc, Join("spec", Escape(k)), "v")
		if err != nil {
			t.Fatalf("set %q: %v", k, err)
		}
		if got := gjson.Get(doc, "spec").Map(); len(got) != 1 || got[k].String() != "v" {
			t.Errorf("key %q: set produced %s", k, doc)
		}
		if got := gjson.Get(doc, Join("spec", Escape(k))).String(); got != "v" {
			t.Errorf("key %q: get returned %q from %s", k, got, doc)
		}
		out, err := sjson.Delete(doc, Join("spec", Escape(k)))
		if err != nil || out != `{"spec":{}}` {
			t.Errorf("key %q: delete gave %q, %v", k, out, err)
		}
	}
}
