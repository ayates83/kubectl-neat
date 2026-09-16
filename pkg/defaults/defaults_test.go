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
package defaults

import (
	"testing"

	"github.com/ayates83/kubectl-neat/pkg/testutil"
	"github.com/tidwall/gjson"
)

func TestComputeDefault(t *testing.T) {
	cases := []struct {
		title  string
		path   string
		data   string
		expect string
	}{
		{
			title: "PullPolicyAlways",
			path:  "spec.containers.0.imagePullPolicy",
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
							"image": "foo",
							"name": "myapp"
						}
					]
				}
			}`,
			expect: "Always",
		},
		{
			title: "PullPolicyIfNotPresent",
			path:  "spec.containers.0.imagePullPolicy",
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
							"image": "foo:bar",
							"name": "myapp"
						}
					]
				}
			}`,
			expect: "IfNotPresent",
		},
		{
			title: "RestartPolicy",
			path:  "spec.restartPolicy",
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
							"image": "foo:bar",
							"name": "myapp"
						}
					]
				}
			}`,
			expect: "Always",
		},
		{
			title: "TerminationMessagePath",
			path:  "spec.containers.0.terminationMessagePath",
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
							"image": "foo:bar",
							"name": "myapp"
						}
					]
				}
			}`,
			expect: "/dev/termination-log",
		},
	}
	for _, c := range cases {
		res, err := computeDefault(c.path, c.data)
		if err != nil {
			t.Errorf("error in computeDefault for case '%s': %v", c.title, err)
		}
		if res != c.expect {
			t.Errorf("test case '%s' failed. want: '%s' have: '%s'", c.title, c.expect, res)
		}
	}
}

func TestIsDefault(t *testing.T) {
	cases := []struct {
		title  string
		path   string
		value  interface{}
		object string
		expect bool
	}{
		{
			title: "PullPolicyAlways",
			path:  "spec.containers.0.imagePullPolicy",
			object: `{
				"apiVersion": "v1",
				"kind": "Pod",
				"metadata": {
					"name": "myapp",
					"namespace": "default"
				},
				"spec": {
					"containers": [
						{
							"image": "foo",
							"name": "myapp"
						}
					]
				}
			}`,
			value:  "Always",
			expect: true,
		},
		{
			title: "PullPolicyIfNotPresent",
			path:  "spec.containers.0.imagePullPolicy",
			object: `{
				"apiVersion": "v1",
				"kind": "Pod",
				"metadata": {
					"name": "myapp",
					"namespace": "default"
				},
				"spec": {
					"containers": [
						{
							"image": "foo:bar",
							"name": "myapp"
						}
					]
				}
			}`,
			value:  "IfNotPresent",
			expect: true,
		},
		{
			title: "RestartPolicy",
			path:  "spec.restartPolicy",
			object: `{
				"apiVersion": "v1",
				"kind": "Pod",
				"metadata": {
					"name": "myapp",
					"namespace": "default"
				},
				"spec": {
					"containers": [
						{
							"image": "foo:bar",
							"name": "myapp"
						}
					]
				}
			}`,
			value:  "Always",
			expect: true,
		},
		{
			title: "TerminationMessagePath",
			path:  "spec.containers.0.terminationMessagePath",
			object: `{
				"apiVersion": "v1",
				"kind": "Pod",
				"metadata": {
					"name": "myapp",
					"namespace": "default"
				},
				"spec": {
					"containers": [
						{
							"image": "foo:bar",
							"name": "myapp"
						}
					]
				}
			}`,
			value:  "/dev/termination-log",
			expect: true,
		},
	}
	for _, c := range cases {
		res, err := isDefault(c.path, c.value, c.object)
		if err != nil {
			t.Errorf("error in isDefault for case '%s': %v", c.title, err)
		}
		if res != c.expect {
			t.Errorf("test case '%s' failed. want: '%v' have: '%v'", c.title, c.expect, res)
		}
	}
}

func TestNeatDefault(t *testing.T) {
	cases := []struct {
		title  string
		data   string
		expect string
	}{
		{
			title: "PullPolicyAlways",
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
							"image": "foo",
							"imagePullPolicy": "Always",
							"name": "myapp"
						}
					]
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
							"image": "foo",
							"name": "myapp"
						}
					]
				}
			}`,
		},
		{
			title: "PullPolicyIfNotPresent",
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
							"image": "foo:bar",
							"imagePullPolicy": "IfNotPresent",
							"name": "myapp"
						}
					]
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
							"image": "foo:bar",
							"name": "myapp"
						}
					]
				}
			}`,
		},
		{
			title: "RestartPolicy",
			data: `{
				"apiVersion": "v1",
				"kind": "Pod",
				"metadata": {
					"name": "myapp",
					"namespace": "default"
				},
				"spec": {
					"restartPolicy": "Always",
					"containers": [
						{
							"image": "foo:bar",
							"name": "myapp"
						}
					]
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
							"image": "foo:bar",
							"name": "myapp"
						}
					]
				}
			}`,
		},
		{
			title: "TerminationMessagePath",
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
							"terminationMessagePath": "/dev/termination-log",
							"image": "foo:bar",
							"name": "myapp"
						}
					]
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
							"image": "foo:bar",
							"name": "myapp"
						}
					]
				}
			}`,
		},
		{
			title: "CRD",
			data: `{
				"apiVersion": "networking.istio.io/v1alpha3",
				"kind": "DestinationRule",
				"metadata": {
					"annotations": {
						"kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"networking.istio.io/v1alpha3\",\"kind\":\"DestinationRule\",\"metadata\":{\"annotations\":{},\"name\":\"default\",\"namespace\":\"one\"},\"spec\":{\"host\":\"*.one.svc.cluster.local\",\"trafficPolicy\":{\"tls\":{\"mode\":\"ISTIO_MUTUAL\"}}}}\n"
					},
					"creationTimestamp": "2019-11-06T20:14:07Z",
					"generation": 1,
					"name": "default",
					"namespace": "one",
					"resourceVersion": "314732",
					"selfLink": "/apis/networking.istio.io/v1alpha3/namespaces/one/destinationrules/default",
					"uid": "fca04858-00d1-11ea-84b3-025000000001"
				},
				"spec": {
					"host": "*.one.svc.cluster.local",
					"trafficPolicy": {
						"tls": {
							"mode": "ISTIO_MUTUAL"
						}
					}
				}
			}`,
			expect: `{
				"apiVersion": "networking.istio.io/v1alpha3",
				"kind": "DestinationRule",
				"metadata": {
					"annotations": {
						"kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"networking.istio.io/v1alpha3\",\"kind\":\"DestinationRule\",\"metadata\":{\"annotations\":{},\"name\":\"default\",\"namespace\":\"one\"},\"spec\":{\"host\":\"*.one.svc.cluster.local\",\"trafficPolicy\":{\"tls\":{\"mode\":\"ISTIO_MUTUAL\"}}}}\n"
					},
					"creationTimestamp": "2019-11-06T20:14:07Z",
					"generation": 1,
					"name": "default",
					"namespace": "one",
					"resourceVersion": "314732",
					"selfLink": "/apis/networking.istio.io/v1alpha3/namespaces/one/destinationrules/default",
					"uid": "fca04858-00d1-11ea-84b3-025000000001"
				},
				"spec": {
					"host": "*.one.svc.cluster.local",
					"trafficPolicy": {
						"tls": {
							"mode": "ISTIO_MUTUAL"
						}
					}
				}
			}`,
		},
	}
	for _, c := range cases {
		resJSON, err := NeatDefaults(c.data)
		if err != nil {
			t.Errorf("error in neatDefaults for case '%s': %v", c.title, err)
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

func TestNeatDefaultsInterdependent(t *testing.T) {
	// completions and parallelism each default to 1 only when the other is absent too.
	// A single pass kept completions; a second run then removed it.
	job := `{"apiVersion":"batch/v1","kind":"Job","metadata":{"name":"once"},"spec":{
		"backoffLimit":6,"completionMode":"NonIndexed","completions":1,"manualSelector":false,"parallelism":1,"suspend":false,
		"template":{"spec":{"containers":[{"name":"c","image":"i"}],"restartPolicy":"Never"}}}}`
	once, err := NeatDefaults(job)
	if err != nil {
		t.Fatal(err)
	}
	twice, err := NeatDefaults(once)
	if err != nil {
		t.Fatal(err)
	}
	if once != twice {
		t.Errorf("not idempotent:\nonce:  %s\ntwice: %s", once, twice)
	}
	for _, p := range []string{"spec.completions", "spec.parallelism", "spec.backoffLimit", "spec.completionMode", "spec.suspend"} {
		if gjson.Get(once, p).Exists() {
			t.Errorf("%s should have been removed: %s", p, once)
		}
	}

	// Authored values that differ from the default must survive, whatever their neighbours are.
	job = `{"apiVersion":"batch/v1","kind":"Job","metadata":{"name":"batch"},"spec":{"completions":5,"parallelism":1,
		"template":{"spec":{"containers":[{"name":"c","image":"i"}],"restartPolicy":"Never"}}}}`
	out, err := NeatDefaults(job)
	if err != nil {
		t.Fatal(err)
	}
	if gjson.Get(out, "spec.completions").Int() != 5 {
		t.Errorf("completions: 5 is authored and must stay: %s", out)
	}
}

func TestPathLess(t *testing.T) {
	if !pathLess("spec.args.2", "spec.args.10") || pathLess("spec.args.10", "spec.args.2") {
		t.Errorf("array indices must compare numerically")
	}
	if !pathLess("spec.a", "spec.a.b") || !pathLess("spec.a.x", "spec.b") {
		t.Errorf("paths must order by segment")
	}
}

func TestNeatDefaultsDottedKeys(t *testing.T) {
	pod := `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"p"},"spec":{
		"nodeSelector":{"kubernetes.io/os":"linux"},
		"containers":[{"name":"c","image":"i","resources":{"limits":{"nvidia.com/gpu":"1"}}}]}}`
	out, err := NeatDefaults(pod)
	if err != nil {
		t.Fatal(err)
	}
	if gjson.Get(out, `spec.nodeSelector.kubernetes\.io/os`).String() != "linux" ||
		gjson.Get(out, `spec.containers.0.resources.limits.nvidia\.com/gpu`).String() != "1" {
		t.Errorf("authored values under dotted keys must stay: %s", out)
	}
	paths, err := flatMapJSON(gjson.Get(pod, "spec").String(), "spec.")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := paths[`spec.nodeSelector.kubernetes\.io/os`]; !ok {
		t.Errorf("flattened paths must escape dotted keys, have %v", paths)
	}
}

func TestNeatDefaultsKeepsListElementsReadable(t *testing.T) {
	np := `{"apiVersion":"networking.k8s.io/v1","kind":"NetworkPolicy","metadata":{"name":"n"},"spec":{"podSelector":{},
		"egress":[{"ports":[{"protocol":"TCP"},{"port":53,"protocol":"TCP"},{"port":53,"protocol":"UDP"}]}],"policyTypes":["Egress"]}}`
	out, err := NeatDefaults(np)
	if err != nil {
		t.Fatal(err)
	}
	if gjson.Get(out, "spec.egress.0.ports.0.protocol").String() != "TCP" {
		t.Errorf("a port that is only {protocol: TCP} must keep it rather than become {}: %s", out)
	}
	if gjson.Get(out, "spec.egress.0.ports.1.protocol").Exists() || gjson.Get(out, "spec.egress.0.ports.2.protocol").String() != "UDP" {
		t.Errorf("elements with other fields still lose the default: %s", out)
	}
}

func TestNeatDefaultsKeepsEmptyStrings(t *testing.T) {
	pod := `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"p"},"spec":{"nodeSelector":{"role":""},
		"containers":[{"name":"c","image":"i","args":["--name",""],"env":[{"name":"EMPTY","value":""}]}]}}`
	out, err := NeatDefaults(pod)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(gjson.Get(out, "spec.containers.0.args").Array()); n != 2 {
		t.Errorf("an empty argument is data and must stay (have %d args): %s", n, out)
	}
	if !gjson.Get(out, "spec.nodeSelector.role").Exists() {
		t.Errorf("an empty label value is data and must stay: %s", out)
	}
	dep := `{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"d"},"spec":{"selector":{"matchLabels":{"tier":""}},
		"template":{"metadata":{"labels":{"tier":""}},"spec":{"containers":[{"name":"c","image":"i"}]}}}}`
	if out, err = NeatDefaults(dep); err != nil {
		t.Fatal(err)
	}
	if !gjson.Get(out, "spec.selector.matchLabels.tier").Exists() || !gjson.Get(out, "spec.template.metadata.labels.tier").Exists() {
		t.Errorf("selector and template labels with empty values must stay: %s", out)
	}
}
