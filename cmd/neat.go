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
	"fmt"
	"strings"

	"github.com/ayates83/kubectl-neat/pkg/defaults"
	"github.com/ayates83/kubectl-neat/pkg/jsonpath"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// Neat gets a Kubernetes resource json as string and de-clutters it to make it more readable.
func Neat(in string) (string, error) {
	var err error
	draft := in

	if in == "" {
		return draft, fmt.Errorf("error in neatPod, input json is empty")
	}
	if !gjson.Valid(in) {
		preview := in
		if len(preview) > 20 {
			preview = preview[:20]
		}
		return draft, fmt.Errorf("error in neatPod, input is not a valid json: %s", preview)
	}

	kind := gjson.Get(in, "kind").String()

	// handle list
	if kind == "List" {
		// Build the items array once: writing each item back with sjson re-parses and copies the
		// whole List every time, which is quadratic in its size.
		var buf strings.Builder
		buf.WriteByte('[')
		for i, item := range gjson.Get(draft, "items").Array() {
			if i > 0 {
				buf.WriteByte(',')
			}
			itemNeat, err := Neat(item.Raw)
			if err != nil {
				itemNeat = item.Raw // keep an item neat cannot handle, as before
			}
			buf.WriteString(itemNeat)
		}
		buf.WriteByte(']')
		if gjson.Get(draft, "items").Exists() {
			if out, err := sjson.SetRaw(draft, "items", buf.String()); err == nil {
				draft = out
			}
		}
		// general neating
		draft, err = neatMetadata(draft)
		if err != nil {
			return draft, fmt.Errorf("error in neatMetadata : %v", err)
		}
		// kubectl writes "metadata": {"resourceVersion": ""} on a List; neatMetadata leaves {},
		// and a List without metadata would gain {} on a second run.
		if m := gjson.Get(draft, "metadata"); !m.Exists() || isResultEmpty(m) {
			draft, _ = sjson.Delete(draft, "metadata")
		}
		return draft, nil
	}

	// Everything that removes whole fields runs before defaulting. Defaulting decodes the entire
	// object, so a field it cannot decode would make it skip the object on this run but not on
	// the next, once that field is gone.
	draft, err = neatStatus(draft)
	if err != nil {
		return draft, fmt.Errorf("error in neatStatus : %v", err)
	}

	// controllers neating
	draft, err = neatScheduler(draft)
	if err != nil {
		return draft, fmt.Errorf("error in neatScheduler : %v", err)
	}
	draft, err = neatServerPopulated(draft)
	if err != nil {
		return draft, fmt.Errorf("error in neatServerPopulated : %v", err)
	}

	// defaults neating
	draft, err = defaults.NeatDefaults(draft)
	if err != nil {
		return draft, fmt.Errorf("error in neatDefaults : %v", err)
	}

	// general neating
	draft, err = neatMetadata(draft)
	if err != nil {
		return draft, fmt.Errorf("error in neatMetadata : %v", err)
	}
	draft, err = neatEmpty(draft)
	if err != nil {
		return draft, fmt.Errorf("error in neatEmpty : %v", err)
	}

	return draft, nil
}

func neatMetadata(in string) (string, error) {
	var err error
	in, err = sjson.Delete(in, `metadata.annotations.kubectl\.kubernetes\.io/last-applied-configuration`)
	if err != nil {
		return in, fmt.Errorf("error deleting last-applied-configuration : %v", err)
	}
	// TODO: prettify this. gjson's @pretty is ok but setRaw the pretty code gives unwanted result
	newMeta := gjson.Get(in, "{metadata.name,metadata.namespace,metadata.labels,metadata.annotations}")
	in, err = sjson.Set(in, "metadata", newMeta.Value())
	if err != nil {
		return in, fmt.Errorf("error setting new metadata : %v", err)
	}
	return in, nil
}

func neatStatus(in string) (string, error) {
	return sjson.Delete(in, "status")
}

func neatScheduler(in string) (string, error) {
	return sjson.Delete(in, "spec.nodeName")
}

// pathPart is one component of a path found by findEmptyPathsRecursive.
type pathPart struct {
	escaped string // path syntax
	key     string // the object key, unescaped; "" for an array element
	element bool   // an array element rather than an object key
}

func joinParts(parts []pathPart) string {
	escaped := make([]string, len(parts))
	for i, p := range parts {
		escaped[i] = p.escaped
	}
	return jsonpath.Join(escaped...)
}

// emptyIsMeaningful reports whether an empty value at the end of parts differs from its absence.
// Kubernetes uses emptiness to mean "everything" in two shapes, and removing them inverts intent:
//   - an empty element of a list: a NetworkPolicy rule {} allows all traffic, while no rule
//     denies it
//   - an empty label selector: {} selects everything, while a missing selector selects nothing
//     (PodDisruptionBudget, topologySpreadConstraints, affinity terms) or the default
//     (namespaceSelector in a NetworkPolicy peer or affinity term)
func emptyIsMeaningful(parts []pathPart) bool {
	last := parts[len(parts)-1]
	return last.element || strings.HasSuffix(last.key, "selector") || strings.HasSuffix(last.key, "Selector")
}

// neatEmpty removes all zero length elements in the json, except where emptiness is meaningful
func neatEmpty(in string) (string, error) {
	var empties [][]pathPart
	findEmptyPathsRecursive(gjson.Parse(in), nil, &empties)
	// Later siblings first: deleting an array element shifts the indices that follow it.
	for e := len(empties) - 1; e >= 0; e-- {
		parts := empties[e]
		// if we just delete the empty path, it may create empty parents
		// so we walk the path and re-check for emptiness at every level
		for i := len(parts); i > 0; i-- {
			if emptyIsMeaningful(parts[:i]) {
				break // neither this nor anything containing it becomes removable
			}
			curPath := joinParts(parts[:i])
			if !isResultEmpty(gjson.Get(in, curPath)) {
				break
			}
			out, err := sjson.Delete(in, curPath)
			if err != nil {
				break // sjson returns "" on error; never let that replace the document
			}
			in = out
		}
	}
	return in, nil
}

// findEmptyPathsRecursive builds a list of paths that point to zero length elements
// cur is the current element to look at
// path is the path components leading to cur
// res is a pointer to a list of empty paths to populate
func findEmptyPathsRecursive(cur gjson.Result, path []pathPart, res *[][]pathPart) {
	if isResultEmpty(cur) {
		if len(path) > 0 {
			*res = append(*res, append([]pathPart(nil), path...))
		}
		return
	}
	if !(cur.IsArray() || cur.IsObject()) {
		return
	}
	// sjson's ForEach doesn't put track index when iterating arrays, hence the index variable
	index := -1
	cur.ForEach(func(k gjson.Result, v gjson.Result) bool {
		var part pathPart
		if cur.IsArray() {
			index++
			part = pathPart{escaped: jsonpath.Index(index), element: true}
		} else {
			part = pathPart{escaped: jsonpath.Escape(k.Str), key: k.Str}
		}
		findEmptyPathsRecursive(v, append(path, part), res)
		return true
	})
}

func isResultEmpty(j gjson.Result) bool {
	v := j.Value()
	switch vt := v.(type) {
	// empty string != lack of string. keep empty strings as it's meaningful data
	// case string:
	// 	return vt == ""
	case []interface{}:
		return len(vt) == 0
	case map[string]interface{}:
		return len(vt) == 0
	}
	return false
}
