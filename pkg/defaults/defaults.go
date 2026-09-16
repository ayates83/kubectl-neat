package defaults

import (
	"encoding/json"
	"fmt"
	goruntime "runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/ayates83/kubectl-neat/pkg/jsonpath"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	admissionregistrationv1 "k8s.io/kubernetes/pkg/apis/admissionregistration/v1"
	appsv1 "k8s.io/kubernetes/pkg/apis/apps/v1"
	autoscalingv1 "k8s.io/kubernetes/pkg/apis/autoscaling/v1"
	autoscalingv2 "k8s.io/kubernetes/pkg/apis/autoscaling/v2"
	batchv1 "k8s.io/kubernetes/pkg/apis/batch/v1"
	certificatesv1 "k8s.io/kubernetes/pkg/apis/certificates/v1"
	corev1 "k8s.io/kubernetes/pkg/apis/core/v1"
	discoveryv1 "k8s.io/kubernetes/pkg/apis/discovery/v1"
	flowcontrolv1 "k8s.io/kubernetes/pkg/apis/flowcontrol/v1"
	networkingv1 "k8s.io/kubernetes/pkg/apis/networking/v1"
	policyv1 "k8s.io/kubernetes/pkg/apis/policy/v1"
	rbacv1 "k8s.io/kubernetes/pkg/apis/rbac/v1"
	resourcev1 "k8s.io/kubernetes/pkg/apis/resource/v1"
	schedulingv1 "k8s.io/kubernetes/pkg/apis/scheduling/v1"
	storagev1 "k8s.io/kubernetes/pkg/apis/storage/v1"
)

// maxPasses bounds the fixpoint loop in NeatDefaults. Real objects settle in two or three.
const maxPasses = 10

// NeatDefaults gets a json document representing a Kubernetes resource, and removes all fields with default values.
// default values is determined by invoking the "defaulting" code from Kubernetes apimachinery
//
// Some defaults depend on each other: a Job's completions and parallelism both default to 1 only
// when both are absent. A single pass tests each field with the others still present, so it can
// stop short (and a second run removes more), or in principle remove two fields that are each
// default alone but not together. NeatDefaults therefore repeats passes until nothing changes, and
// accepts a pass only if defaulting the result gives back every removed field's original value.
func NeatDefaults(in string) (string, error) {
	var pom metav1.PartialObjectMetadata
	if err := json.Unmarshal([]byte(in), &pom); err != nil {
		return "", fmt.Errorf("error unmarshaling as PartialObject : %v", err)
	}
	if !myscheme.Recognizes(pom.GroupVersionKind()) {
		return in, nil
	}
	if !gjson.Get(in, "spec").Exists() {
		return in, nil
	}

	removed := map[string]interface{}{} // every path removed so far, with its value in the input
	for pass := 0; pass < maxPasses; pass++ {
		candidates, err := defaultPaths(in)
		if err != nil {
			return "", err
		}
		if len(candidates) == 0 {
			break
		}
		out, accepted := removeVerified(in, candidates, removed)
		if len(accepted) == 0 {
			break
		}
		for p, v := range accepted {
			removed[p] = v
		}
		in = out
	}
	return in, nil
}

// defaultPaths returns the leaf paths under spec whose value equals what defaulting would set
// if that one field were absent.
func defaultPaths(in string) (map[string]interface{}, error) {
	paths, err := flatMapJSON(gjson.Get(in, "spec").String(), "spec.")
	if err != nil {
		return nil, fmt.Errorf("error flattening json : %v", err)
	}
	// Each check decodes and defaults the whole object, and they are independent: run them
	// concurrently. The scheme and decoder are read-only after init.
	keys := sortedKeys(paths)
	results := make([]bool, len(keys))
	errs := make([]error, len(keys))
	var wg sync.WaitGroup
	next := make(chan int)
	for w := 0; w < goruntime.GOMAXPROCS(0); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				results[i], errs[i] = isDefault(keys[i], paths[keys[i]], in)
			}
		}()
	}
	for i := range keys {
		next <- i
	}
	close(next)
	wg.Wait()
	for i, k := range keys {
		if errs[i] != nil {
			log.Error(fmt.Errorf("error determining default for '%s' : %v", k, errs[i]))
		}
		if errs[i] != nil || !results[i] {
			delete(paths, k)
		}
	}
	keepListElementsReadable(in, paths)
	return paths, nil
}

// keepListElementsReadable drops candidates that would leave a list element as {}.
// The element must stay (an empty NetworkPolicy port still means "TCP, any port"), and
// {"protocol": "TCP"} says that where {} does not.
func keepListElementsReadable(in string, candidates map[string]interface{}) {
	byParent := map[string][]string{}
	for p := range candidates {
		if i := strings.LastIndex(p, "."); i > 0 && !strings.HasSuffix(p[:i], `\`) {
			byParent[p[:i]] = append(byParent[p[:i]], p)
		}
	}
	for parent, children := range byParent {
		i := strings.LastIndex(parent, ".")
		if i < 0 {
			continue
		}
		if _, err := strconv.Atoi(parent[i+1:]); err != nil || !gjson.Get(in, parent[:i]).IsArray() {
			continue // not a list element
		}
		if obj := gjson.Get(in, parent); obj.IsObject() && len(obj.Map()) == len(children) {
			for _, c := range children {
				delete(candidates, c)
			}
		}
	}
}

// removeVerified removes candidates from in, keeping only removals that defaulting reverses.
// It first tries all candidates at once; if the result does not default back to the original
// values of everything removed so far, it adds candidates one at a time in a fixed order.
func removeVerified(in string, candidates, removedBefore map[string]interface{}) (string, map[string]interface{}) {
	all, ok := deletePaths(in, sortedKeys(candidates))
	if ok && restores(all, removedBefore, candidates) {
		return all, candidates
	}
	accepted := map[string]interface{}{}
	for _, p := range sortedKeys(candidates) {
		next, ok := deletePaths(in, []string{p})
		if !ok || !restores(next, removedBefore, accepted, map[string]interface{}{p: candidates[p]}) {
			continue
		}
		in = next
		accepted[p] = candidates[p]
	}
	return in, accepted
}

// deletePaths deletes paths deepest-first, so removing an array element cannot shift the index
// of another path in the same array.
func deletePaths(in string, paths []string) (string, bool) {
	var err error
	for i := len(paths) - 1; i >= 0; i-- {
		if in, err = sjson.Delete(in, paths[i]); err != nil {
			log.Error(fmt.Errorf("error deleting default '%s' : %v", paths[i], err))
			return "", false
		}
	}
	return in, true
}

// restores reports whether defaulting obj sets every path in want back to its value.
func restores(obj string, want ...map[string]interface{}) bool {
	defaulted, err := applyDefaults(obj)
	if err != nil {
		log.Error(fmt.Errorf("error verifying defaults : %v", err))
		return false
	}
	for _, m := range want {
		for p, v := range m {
			if r := gjson.Get(defaulted, p); !r.Exists() || r.String() != fmt.Sprintf("%v", v) {
				return false
			}
		}
	}
	return true
}

// sortedKeys orders paths segment by segment, comparing array indices numerically, so that
// spec.args.2 sorts before spec.args.10 and deleting in reverse order never shifts an index.
func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return pathLess(keys[i], keys[j]) })
	return keys
}

func pathLess(a, b string) bool {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(as) && i < len(bs); i++ {
		if as[i] == bs[i] {
			continue
		}
		an, aerr := strconv.Atoi(as[i])
		bn, berr := strconv.Atoi(bs[i])
		if aerr == nil && berr == nil {
			return an < bn
		}
		return as[i] < bs[i]
	}
	return len(as) < len(bs)
}

// flatMapJSON gets a json document and builds a map of all the leaf keys and their values.
// Keys are escaped path components, so a key like "kubernetes.io/os" is addressed exactly.
// Empty objects and arrays have no leaves and produce no entries.
func flatMapJSON(j string, prefix string) (map[string]interface{}, error) {
	var parsed interface{}
	if err := json.Unmarshal([]byte(j), &parsed); err != nil {
		return nil, fmt.Errorf("error unmarshaling: %v", err)
	}
	if _, ok := parsed.(map[string]interface{}); !ok {
		return nil, fmt.Errorf("error flattening json : not an object")
	}
	res := map[string]interface{}{}
	flattenInto(res, strings.TrimSuffix(prefix, "."), parsed)
	return res, nil
}

func flattenInto(res map[string]interface{}, path string, v interface{}) {
	child := func(part string) string {
		if path == "" {
			return part
		}
		return jsonpath.Join(path, part)
	}
	switch vt := v.(type) {
	case map[string]interface{}:
		for k, c := range vt {
			flattenInto(res, child(jsonpath.Escape(k)), c)
		}
	case []interface{}:
		for i, c := range vt {
			switch c.(type) {
			case map[string]interface{}, []interface{}:
				flattenInto(res, child(jsonpath.Index(i)), c)
			default:
				// A scalar list element (an argument, a finalizer) is never a default candidate:
				// defaulting does not fill in list items, and deleting one shifts the rest.
			}
		}
	default:
		res[path] = v
	}
}

var myscheme *runtime.Scheme
var decoder runtime.Decoder

// schemeAdders registers the types and the server-side defaulting functions of every
// built-in API group that has them. Kinds outside this list are left untouched by
// NeatDefaults, because there is nothing authoritative to compare against.
var schemeAdders = []func(*runtime.Scheme) error{
	corev1.AddToScheme,
	appsv1.AddToScheme,
	batchv1.AddToScheme,
	autoscalingv1.AddToScheme,
	autoscalingv2.AddToScheme,
	networkingv1.AddToScheme,
	policyv1.AddToScheme,
	rbacv1.AddToScheme,
	storagev1.AddToScheme,
	schedulingv1.AddToScheme,
	discoveryv1.AddToScheme,
	certificatesv1.AddToScheme,
	flowcontrolv1.AddToScheme,
	admissionregistrationv1.AddToScheme,
	resourcev1.AddToScheme,
}

func init() {
	myscheme = runtime.NewScheme()
	for _, add := range schemeAdders {
		if err := add(myscheme); err != nil {
			panic(fmt.Sprintf("registering defaulting scheme: %v", err))
		}
	}
	decoder = scheme.Codecs.UniversalDeserializer()
}

// isDefault determins if the observed 'value' of the 'path' (gjson path) to field  in 'objJSON' is a default value
func isDefault(path string, value interface{}, objJSON string) (bool, error) {
	computed, set, err := defaultFor(path, objJSON)
	if err != nil {
		return false, fmt.Errorf("error computing default for '%s' : %v", path, err)
	}
	// A field defaulting leaves unset is not default, even when its value reads as "": an empty
	// label value or argument is data.
	return set && computed == fmt.Sprintf("%v", value), nil
}

// computeDefault returns the default value for the 'path' (gjson path) to field in 'objJSON'
func computeDefault(path string, objJSON string) (string, error) {
	value, _, err := defaultFor(path, objJSON)
	return value, err
}

// defaultFor returns the value defaulting sets at path when the field is absent, and whether it
// sets one at all.
func defaultFor(path string, objJSON string) (string, bool, error) {
	candidateJSON, err := sjson.Delete(objJSON, path)
	if err != nil {
		return "", false, fmt.Errorf("error deleting path to default '%s' : %v", path, err)
	}
	resJSON, err := applyDefaults(candidateJSON)
	if err != nil {
		return "", false, err
	}
	r := gjson.Get(resJSON, path)
	return r.String(), r.Exists(), nil
}

// applyDefaults decodes objJSON, runs the API server's defaulting functions on it, and
// returns the result as JSON.
func applyDefaults(objJSON string) (string, error) {
	obj, _, err := decoder.Decode([]byte(objJSON), nil, nil)
	if err != nil {
		return "", fmt.Errorf("error decoding into kubernetes object : %v", err)
	}
	myscheme.Default(obj)
	resJSON, err := json.Marshal(obj)
	if err != nil {
		return "", fmt.Errorf("error marshaling kubernetes object : %v", err)
	}
	return string(resJSON), nil
}
