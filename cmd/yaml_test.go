package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ghodss/yaml"
)

func TestJSONToYAMLIsDeterministic(t *testing.T) {
	// go-yaml's own ordering of these keys varies between runs.
	in := []byte(`{"data":{"00-base.conf":"a","010-extra.conf":"b","001-schema.sql":"c","01-init.sql":"d","1-a.conf":"e","0a.conf":"f","002b":"g","02a":"h"}}`)
	first, err := jsonToYAML(in)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 200; i++ {
		if out, _ := jsonToYAML(in); string(out) != string(first) {
			t.Fatalf("run %d differs:\n%s\nvs\n%s", i, out, first)
		}
	}
}

func TestJSONToYAMLMatchesGoYAMLForOrdinaryKeys(t *testing.T) {
	files, _ := filepath.Glob("../test/fixtures/*.json")
	files = append(files, "")
	extra := `{"b":1,"a10":2,"a2":3,"A":true,"z":[{"y":null,"x":1.5}],"port":8080,"1":"one"}`
	for _, f := range files {
		in := []byte(extra)
		if f != "" {
			var err error
			if in, err = os.ReadFile(f); err != nil {
				t.Fatal(err)
			}
		}
		want, err := yaml.JSONToYAML(in)
		if err != nil {
			t.Fatal(err)
		}
		have, err := jsonToYAML(in)
		if err != nil {
			t.Fatal(err)
		}
		if string(have) != string(want) {
			t.Errorf("%s: output differs from ghodss/yaml:\nwant %s\nhave %s", f, want, have)
		}
	}
}
