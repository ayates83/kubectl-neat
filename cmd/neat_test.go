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
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ayates83/kubectl-neat/pkg/testutil"
	"github.com/tidwall/gjson"
)

func TestNeatMetadata(t *testing.T) {
	cases := []struct {
		title  string
		data   string
		expect string
	}{
		{
			title: "pod metadata",
			data: `{
				"metadata": {
					"creationTimestamp": "2019-04-24T19:55:27Z",
					"labels": {
						"name": "myapp"
					},
					"name": "myapp",
					"namespace": "default",
					"resourceVersion": "274103",
					"selfLink": "/api/v1/namespaces/default/pods/myapp",
					"uid": "e8330f3c-66ca-11e9-b6fa-0800271788ca"
				}
			}`,
			expect: `{
				"metadata": {
					"labels": {
						"name": "myapp"
					},
					"name": "myapp",
					"namespace": "default"
				}
			}`,
		},
		{
			title: "annotations with apply",
			data: `{
				"metadata": {
					"name": "test",
					"namespace": "testns",
					"annotations": {
						"my-annotation": "is here",
						"kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"authentication.istio.io/v1alpha1\",\"kind\":\"Policy\",\"metadata\":{\"annotations\":{},\"name\":\"default\",\"namespace\":\"one\"},\"spec\":{\"peers\":[{\"mtls\":{}}]}}\n"
					}
				}
			}`,
			expect: `{
				"metadata": {
					"name": "test",
					"namespace": "testns",
					"annotations": {
						"my-annotation": "is here"
					}
				}
			}`,
		},
	}
	for _, c := range cases {
		resJSON, err := neatMetadata(c.data)
		if err != nil {
			t.Errorf("error in neatMetadata for case '%s': %v", c.title, err)
			continue
		}
		equal, err := testutil.JSONEqual(resJSON, c.expect)
		if err != nil {
			t.Errorf("error in JSONEqual for case '%s': %v", c.title, err)
			continue
		}
		if !equal {
			t.Errorf("test case '%s' failed. want: '%s' have: '%s'", c.title, c.expect, resJSON)
		}

	}
}

func TestNeatScheduler(t *testing.T) {
	cases := []struct {
		title  string
		data   string
		expect string
	}{
		{
			title: "nodeName",
			data: `{
				"apiVersion": "v1",
				"kind": "Pod",
				"metadata": {
					"name": "myapp",
					"namespace": "default"
				},
				"spec": {
					"containers": [
						{
							"image": "nginx",
							"imagePullPolicy": "Always",
							"name": "myapp"
						}
					],
					"nodeName": "minikube"
				}
			}`,
			expect: `{
				"apiVersion": "v1",
				"kind": "Pod",
				"metadata": {
					"name": "myapp",
					"namespace": "default"
				},
				"spec": {
					"containers": [
						{
							"image": "nginx",
							"imagePullPolicy": "Always",
							"name": "myapp"
						}
					]
				}
			}`,
		},
	}
	for _, c := range cases {
		resJSON, err := neatScheduler(c.data)
		if err != nil {
			t.Errorf("error in neatScheduler for case '%s': %v", c.title, err)
			continue
		}
		equal, err := testutil.JSONEqual(resJSON, c.expect)
		if err != nil {
			t.Errorf("error in JSONEqual for case '%s': %v", c.title, err)
			continue
		}
		if !equal {
			t.Errorf("test case '%s' failed. want: '%s' have: '%s'", c.title, c.expect, resJSON)
		}

	}
}

func TestNeatServiceAccount(t *testing.T) {
	cases := []struct {
		title  string
		data   string
		expect string
	}{
		{
			title: "pod multi volumes",
			data: `{
				"apiVersion": "v1",
				"kind": "Pod",
				"metadata": {
					"labels": {
						"name": "myapp"
					},
					"name": "myapp",
					"namespace": "default"
				},
				"spec": {
					"containers": [
						{
							"image": "nginx",
							"name": "myapp",
							"volumeMounts": [
								{
									"mountPath": "/my",
									"name": "my",
									"readOnly": false
								},
								{
									"mountPath": "/var/run/secrets/kubernetes.io/serviceaccount",
									"name": "default-token-nmshj",
									"readOnly": true
								}						
							]
						}
					],
					"serviceAccount": "default",
					"serviceAccountName": "default",
					"volumes": [
						{
							"name": "default-token-nmshj",
							"secret": {
								"defaultMode": 420,
								"secretName": "default-token-nmshj"
							}
						},
						{
							"name": "my",
							"hostPath": {
								"path": "/my",
								"type": "Directory"
							}
						}
					]
				}
			}`,
			expect: `{
				"apiVersion": "v1",
				"kind": "Pod",
				"metadata": {
					"labels": {
						"name": "myapp"
					},
					"name": "myapp",
					"namespace": "default"
				},
				"spec": {
					"containers": [
						{
							"image": "nginx",
							"name": "myapp",
							"volumeMounts": [
								{
									"mountPath": "/my",
									"name": "my",
									"readOnly": false
								}						
							]
						}
					],
					"serviceAccountName": "default",
					"volumes": [
						{
							"name": "my",
							"hostPath": {
								"path": "/my",
								"type": "Directory"
							}
						}
					]
				}
			}`,
		},
	}
	for _, c := range cases {
		resJSON, err := neatServiceAccountToken(c.data)
		if err != nil {
			t.Errorf("error in neatServiceAccountToken for case '%s': %v", c.title, err)
			continue
		}
		equal, err := testutil.JSONEqual(resJSON, c.expect)
		if err != nil {
			t.Errorf("error in JSONEqual for case '%s': %v", c.title, err)
			continue
		}
		if !equal {
			t.Errorf("test case '%s' failed. want: '%s' have: '%s'", c.title, c.expect, resJSON)
		}

	}
}

func TestNeatEmpty(t *testing.T) {
	cases := []struct {
		title  string
		data   string
		expect string
	}{
		{
			title:  "empty object",
			data:   `{ "foo": "bar", "baz": {} }`,
			expect: `{ "foo": "bar"}`,
		},
		{
			title:  "empty array",
			data:   `{ "foo": "bar", "baz": [] }`,
			expect: `{ "foo": "bar"}`,
		},
		{
			// An empty list element can mean "everything" (a NetworkPolicy rule {}), so it stays.
			title:  "empty second arrray element",
			data:   `{ "foo": [ "bar", {} ] }`,
			expect: `{ "foo": [ "bar", {} ] }`,
		},
		{
			title:  "empty array object",
			data:   `{ "foo": "bar", "baz": { [] } }`,
			expect: `{ "foo": "bar"}`,
		},
		{
			title:  "single empty array in object",
			data:   `{ "foo": "bar", "baz": { "fiz": [] } }`,
			expect: `{ "foo": "bar"}`,
		},
	}
	for _, c := range cases {
		resJSON, err := neatEmpty(c.data)
		if err != nil {
			t.Errorf("error in Neat for case '%s': %v", c.title, err)
			continue
		}
		equal, err := testutil.JSONEqual(resJSON, c.expect)
		if err != nil {
			t.Errorf("error in JSONEqual for case '%s': %v", c.title, err)
			continue
		}
		if !equal {
			t.Errorf("test case '%s' failed. want: '%s' have: '%s'", c.title, c.expect, resJSON)
		}

	}
}

func TestNeat(t *testing.T) {
	testsDir := "../test/fixtures"
	testFiles, err := ioutil.ReadDir(testsDir)
	if err != nil {
		t.Fatalf("can't list tests in: %s", testsDir)
	}
	for _, f := range testFiles {
		fName := f.Name()
		fParts := strings.Split(fName, "-")
		if fParts[1] == "raw.json" {
			fFullName := filepath.Join(testsDir, f.Name())
			inBytes, err := ioutil.ReadFile(fFullName)
			if err != nil {
				t.Errorf("can't read file: %s", fFullName)
			}
			expFullName := filepath.Join(testsDir, fParts[0]+"-neat.json")
			expBytes, err := ioutil.ReadFile(expFullName)
			if err != nil {
				t.Errorf("can't read file: %s", expFullName)
			}
			resJSON, err := Neat(string(inBytes))
			if err != nil {
				t.Errorf("error in Neat for case: %s: %v", fName, err)
				continue
			}
			equal, err := testutil.JSONEqual(resJSON, string(expBytes))
			if err != nil {
				t.Errorf("error in JSONEqual for case: %s: %v", fName, err)
				continue
			}
			if !equal {
				t.Errorf("test case failed: %s:\nhave %s\nwant %s", fName, resJSON, string(expBytes))
			}
		}
	}
}

func TestNeatEmptyKeysAndArrays(t *testing.T) {
	cases := []struct{ title, data, want string }{
		{
			// sjson.Delete returns "" on a bad path; neatEmpty used to assign it over the document.
			title: "a key that is path syntax does not wipe the document",
			data:  `{"kind":"X","spec":{"#000":{"":{}},"keep":1}}`,
			want:  `{"kind":"X","spec":{"keep":1}}`,
		},
		{
			title: "a dotted key is addressed exactly",
			data:  `{"spec":{"app.config":{},"app":{"config":"x"}}}`,
			want:  `{"spec":{"app":{"config":"x"}}}`,
		},
		{
			title: "empty list elements stay, and empty values inside them still go",
			data:  `{"spec":{"items":[{},{"a":{}},1,{},[]],"other":[]}}`,
			want:  `{"spec":{"items":[{},{},1,{},[]]}}`,
		},
	}
	for _, c := range cases {
		have, err := neatEmpty(c.data)
		if err != nil {
			t.Fatalf("%s: %v", c.title, err)
		}
		if equal, err := testutil.JSONEqual(have, c.want); err != nil || !equal {
			t.Errorf("%s:\nwant %s\nhave %s (%v)", c.title, c.want, have, err)
		}
	}
}

func TestNeatListWithoutMetadataIsIdempotent(t *testing.T) {
	in := `{"apiVersion":"v1","kind":"List","items":[{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"a"}}]}`
	once, err := Neat(in)
	if err != nil {
		t.Fatal(err)
	}
	twice, err := Neat(once)
	if err != nil {
		t.Fatal(err)
	}
	if equal, _ := testutil.JSONEqual(once, twice); !equal || gjson.Get(once, "metadata").Exists() {
		t.Errorf("want no metadata and a stable result:\nonce  %s\ntwice %s", once, twice)
	}
}

func TestNeatKeepsMeaningfulEmptiness(t *testing.T) {
	cases := []struct {
		title, data string
		keep        []string
	}{
		{
			title: "NetworkPolicy: an empty egress rule allows all egress; without it, all egress is denied",
			data:  `{"apiVersion":"networking.k8s.io/v1","kind":"NetworkPolicy","metadata":{"name":"n"},"spec":{"podSelector":{},"egress":[{}],"policyTypes":["Egress"]}}`,
			keep:  []string{"spec.egress.0", "spec.podSelector"},
		},
		{
			title: "NetworkPolicy: an empty namespaceSelector admits every namespace",
			data:  `{"apiVersion":"networking.k8s.io/v1","kind":"NetworkPolicy","metadata":{"name":"n"},"spec":{"podSelector":{"matchLabels":{"app":"a"}},"ingress":[{"from":[{"namespaceSelector":{}}]}],"policyTypes":["Ingress"]}}`,
			keep:  []string{"spec.ingress.0.from.0.namespaceSelector"},
		},
		{
			title: "NetworkPolicy: a port with the default protocol still restricts the rule to TCP",
			data:  `{"apiVersion":"networking.k8s.io/v1","kind":"NetworkPolicy","metadata":{"name":"n"},"spec":{"podSelector":{},"egress":[{"ports":[{"protocol":"TCP"}]}],"policyTypes":["Egress"]}}`,
			keep:  []string{"spec.egress.0.ports.0"},
		},
		{
			title: "PodDisruptionBudget: an empty selector covers every pod; none covers no pod",
			data:  `{"apiVersion":"policy/v1","kind":"PodDisruptionBudget","metadata":{"name":"p"},"spec":{"maxUnavailable":1,"selector":{}}}`,
			keep:  []string{"spec.selector"},
		},
		{
			title: "Pod: empty selectors in affinity and topology spread select every pod",
			data: `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"p"},"spec":{"containers":[{"name":"c","image":"i"}],
				"topologySpreadConstraints":[{"maxSkew":1,"topologyKey":"zone","whenUnsatisfiable":"DoNotSchedule","labelSelector":{}}],
				"affinity":{"podAntiAffinity":{"requiredDuringSchedulingIgnoredDuringExecution":[{"topologyKey":"kubernetes.io/hostname","labelSelector":{},"namespaceSelector":{}}]}}}}`,
			keep: []string{"spec.topologySpreadConstraints.0.labelSelector",
				"spec.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution.0.labelSelector",
				"spec.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution.0.namespaceSelector"},
		},
	}
	for _, c := range cases {
		out, err := Neat(c.data)
		if err != nil {
			t.Fatalf("%s: %v", c.title, err)
		}
		for _, p := range c.keep {
			if !gjson.Get(out, p).Exists() {
				t.Errorf("%s: %s was removed:\n%s", c.title, p, out)
			}
		}
	}

	// emptyDir: {} is not meaningful: defaulting turns a volume without a source into one.
	out, err := Neat(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"p"},"spec":{"containers":[{"name":"c","image":"i"}],"volumes":[{"name":"v","emptyDir":{}}]}}`)
	if err != nil {
		t.Fatal(err)
	}
	if gjson.Get(out, "spec.volumes.0.emptyDir").Exists() {
		t.Errorf("emptyDir: {} should still be removed: %s", out)
	}
}
