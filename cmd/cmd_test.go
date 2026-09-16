/*
Copyright © 2019 Itay Shakury @itaysk

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func assertErrorNil(err error) bool {
	return err == nil
}
func TestRootCmd(t *testing.T) {
	resourceDataJSONPath := "../test/fixtures/service1-raw.json"
	resourceDataJSONBytes, err := ioutil.ReadFile(resourceDataJSONPath)
	resourceDataJSON := string(resourceDataJSONBytes)
	if err != nil {
		t.Errorf("error readin test data file %s: %v", resourceDataJSONPath, err)
	}
	resourceDataYAMLPath := "../test/fixtures/service1-raw.yaml"
	resourceDataYAMLBytes, err := ioutil.ReadFile(resourceDataYAMLPath)
	resourceDataYAML := string(resourceDataYAMLBytes)
	if err != nil {
		t.Errorf("error readin test data file %s: %v", resourceDataYAMLPath, err)
	}

	testcases := []struct {
		args        []string
		stdin       string
		assertError func(err error) bool
		expOut      string
	}{
		{
			args:        []string{},
			stdin:       "",
			assertError: assertErrorNil,
			expOut:      "",
		},
		{
			args:        []string{},
			stdin:       resourceDataJSON,
			assertError: assertErrorNil,
			expOut:      "apiVersion",
		},
		{
			args:        []string{},
			stdin:       resourceDataYAML,
			assertError: assertErrorNil,
			expOut:      "apiVersion",
		},
		{
			args:        []string{"-f", "-"},
			stdin:       resourceDataJSON,
			assertError: assertErrorNil,
			expOut:      "apiVersion",
		},
		{
			args:  []string{"-f", "/nogood"},
			stdin: "",
			assertError: func(err error) bool {
				_, ok := err.(*os.PathError)
				return ok
			},
			expOut: "",
		},
		{
			args:        []string{"-f", resourceDataJSONPath},
			stdin:       "",
			assertError: assertErrorNil,
			expOut:      "apiVersion",
		},
		{
			args:        []string{"-f", resourceDataYAMLPath},
			stdin:       "",
			assertError: assertErrorNil,
			expOut:      "apiVersion",
		},
	}

	for _, tc := range testcases {
		rootCmd.SetArgs(tc.args)
		if tc.stdin != "" {
			rootCmd.SetIn(bytes.NewReader([]byte(tc.stdin)))
		}
		cmdout := new(bytes.Buffer)
		cmderr := new(bytes.Buffer)
		rootCmd.SetOut(cmdout)
		rootCmd.SetErr(cmderr)
		rootCmd.ParseFlags(tc.args)
		resErr := rootCmd.RunE(rootCmd, tc.args)
		resStdout, err := ioutil.ReadAll(cmdout)
		if err != nil {
			t.Errorf("error reading command output: %v", err)
		}
		resStderr, err := ioutil.ReadAll(cmderr)
		if err != nil {
			t.Errorf("error reading command error: %v\ntest case: %v", err, tc)
		}
		if tc.assertError != nil && !tc.assertError(resErr) {
			t.Errorf("error assertion: have: %#v\ntest case: %v", resErr, tc)
		}
		if !strings.Contains(string(resStdout), tc.expOut) {
			t.Errorf("stdout assertion: have: %s\nwant: %s\ntest case: %v", string(resStdout), tc.expOut, tc)
		}
		if len(resStderr) > 0 {
			t.Errorf("stderr not empty: %s\ntest case: %v", string(resStderr), tc)
		}
	}
}

func TestGetCmd(t *testing.T) {
	kubectl = "../test/kubectl-stub"
	testcases := []struct {
		args        []string
		assertError func(err error) bool
		expOut      string
		expErr      string
	}{
		{
			args: []string{""},
			assertError: func(err error) bool {
				return strings.HasPrefix(err.Error(), "Error invoking kubectl")
			},
			expOut: "",
			expErr: "",
		},
		{
			args:        []string{"pods"},
			assertError: assertErrorNil,
			expOut:      "apiVersion",
			expErr:      "",
		},
		{
			args:        []string{"pods", "mypod"},
			assertError: assertErrorNil,
			expOut:      "apiVersion",
			expErr:      "",
		},
		{
			args:        []string{"pods", "mypod", "-o", "yaml"},
			assertError: assertErrorNil,
			expOut:      "apiVersion",
			expErr:      "",
		},
		{
			args:        []string{"pods", "mypod", "-o", "json"},
			assertError: assertErrorNil,
			expOut:      "apiVersion",
			expErr:      "",
		},
	}

	for _, tc := range testcases {
		rootCmd.SetArgs(tc.args)
		cmdout := new(bytes.Buffer)
		cmderr := new(bytes.Buffer)
		rootCmd.SetOut(cmdout)
		rootCmd.SetErr(cmderr)
		rootCmd.ParseFlags(tc.args)
		resErr := getCmd.RunE(getCmd, tc.args)
		resStdout, err := ioutil.ReadAll(cmdout)
		if err != nil {
			t.Errorf("error reading command output: %v", err)
		}
		resStderr, err := ioutil.ReadAll(cmderr)
		if err != nil {
			t.Errorf("error reading command error: %v\ntest case: %v", err, tc)
		}
		if tc.assertError != nil && !tc.assertError(resErr) {
			t.Errorf("error assertion: have: %#v\ntest case: %v", resErr, tc)
		}
		if !strings.Contains(string(resStdout), tc.expOut) {
			t.Errorf("stdout assertion: have: %s\nwant: %s\ntest case: %v", string(resStdout), tc.expOut, tc)
		}
		if len(resStderr) > 0 {
			t.Errorf("stderr not empty: %s\ntest case: %v", string(resStderr), tc)
		}
	}
}

func TestNeatYAMLOrJSONMultiDocument(t *testing.T) {
	stream := `---
# leading separator and a comment-only document must not produce output
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: first
  uid: 00000000-0000-0000-0000-000000000001
data:
  script: |
    echo start
    ---
    echo a separator inside a block scalar is data
---   # a separator may carry a comment
apiVersion: v1
kind: Service
metadata:
  name: second
spec:
  clusterIP: 10.0.0.1
  ports:
  - port: 80
`
	out, err := NeatYAMLOrJSON([]byte(stream), "same")
	if err != nil {
		t.Fatalf("multi-document yaml: %v", err)
	}
	docs, err := splitYAMLDocuments(out)
	if err != nil {
		t.Fatalf("re-reading output: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("want 2 documents, have %d:\n%s", len(docs), out)
	}
	for _, unwanted := range []string{"uid:", "clusterIP:"} {
		if strings.Contains(string(out), unwanted) {
			t.Errorf("%q should have been neated from every document:\n%s", unwanted, out)
		}
	}
	if !strings.Contains(string(out), "    ---\n    echo a separator") {
		t.Errorf("block scalar content was split or altered:\n%s", out)
	}

	out, err = NeatYAMLOrJSON([]byte(stream), "json")
	if err != nil {
		t.Fatalf("multi-document yaml to json: %v", err)
	}
	var list struct {
		Kind  string            `json:"kind"`
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(out, &list); err != nil || list.Kind != "List" || len(list.Items) != 2 {
		t.Errorf("want a v1 List holding both documents, have:\n%s", out)
	}

	if _, err = NeatYAMLOrJSON([]byte("a: 1\n--- b: 2\n"), "same"); err == nil {
		t.Errorf("want an error for content after a separator")
	}
}

func TestNeatShortInvalidJSON(t *testing.T) {
	// used to panic slicing in[:20]
	if _, err := Neat("{bad"); err == nil {
		t.Errorf("want an error for invalid json")
	}
}

func TestKubectlOutputFormat(t *testing.T) {
	cases := map[string][]string{
		"":     {"pod", "json"}, // a resource named json
		"json": {"pod", "p", "-o", "json"},
		"yaml": {"pod", "p", "-oyaml"},
		"wide": {"pod", "--output=wide"},
	}
	cases["json "] = []string{"pod", "-o=json"}
	cases["yaml "] = []string{"-ojson", "pod", "--output", "yaml"} // last one wins
	for want, args := range cases {
		if have := kubectlOutputFormat(args); have != strings.TrimSpace(want) {
			t.Errorf("kubectlOutputFormat(%q) = %q, want %q", args, have, strings.TrimSpace(want))
		}
	}
}

func TestGetIgnoresKubectlWarnings(t *testing.T) {
	stub, err := filepath.Abs("../test/kubectl-stub")
	if err != nil {
		t.Fatal(err)
	}
	noisy := filepath.Join(t.TempDir(), "kubectl")
	script := "#!/usr/bin/env bash\necho 'Warning: v1 ComponentStatus is deprecated' >&2\nexec " + stub + " \"$@\"\n"
	if err := os.WriteFile(noisy, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	defer func(k string) { kubectl = k }(kubectl)
	kubectl = noisy

	var stdout, stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	defer func() { rootCmd.SetOut(os.Stdout); rootCmd.SetErr(os.Stderr); rootCmd.SetArgs(nil) }()
	output := rootCmd.PersistentFlags().Lookup("output") // flag state outlives earlier Execute calls
	output.Value.Set("yaml")
	output.Changed = false
	rootCmd.SetArgs([]string{"get", "--", "pods", "mypod"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("a warning on kubectl's stderr must not break parsing: %v", err)
	}
	if !strings.HasPrefix(stdout.String(), "apiVersion") {
		t.Errorf("want neated yaml on stdout, have:\n%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "Warning: v1 ComponentStatus is deprecated") {
		t.Errorf("kubectl's warning should be passed through on stderr, have: %q", stderr.String())
	}
}

func TestYAMLStreamStartingWithFlowMapping(t *testing.T) {
	out, err := NeatYAMLOrJSON([]byte("{}\n---\napiVersion: v1\nkind: ConfigMap\nmetadata: {name: a}\n"), "same")
	if err != nil {
		t.Fatalf("a YAML stream whose first document is {} is not JSON: %v", err)
	}
	if !strings.Contains(string(out), "kind: ConfigMap") {
		t.Errorf("want the ConfigMap document in YAML, have:\n%s", out)
	}
}
