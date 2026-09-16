/*
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
	"sort"
	"unicode"

	yamlv2 "gopkg.in/yaml.v2"
)

// jsonToYAML is github.com/ghodss/yaml.JSONToYAML with a deterministic key order.
//
// go-yaml v2 sorts map keys "naturally" (a2 before a10) with sort.Sort over Go's randomised
// map iteration, and its comparison is not a consistent order when digit runs have leading
// zeros. The same object can then print with keys in a different order on every run --
// ConfigMap data keys such as 00-base.conf and 010-extra.conf are enough, and kubectl -o yaml
// shows it too. Here the keys are put in a fixed order first and sorted stably with the same
// comparison, so identical input always gives identical output, and any key set go-yaml
// orders consistently comes out exactly as it would from kubectl.
func jsonToYAML(j []byte) ([]byte, error) {
	var obj interface{}
	// yaml.Unmarshal rather than json.Unmarshal keeps integer types, as ghodss/yaml does.
	if err := yamlv2.Unmarshal(j, &obj); err != nil {
		return nil, err
	}
	return yamlv2.Marshal(orderedYAML(obj))
}

// orderedYAML replaces every map with a yaml.MapSlice, which go-yaml writes in the given order.
func orderedYAML(v interface{}) interface{} {
	switch vt := v.(type) {
	case map[interface{}]interface{}:
		keys := make([]interface{}, 0, len(vt))
		for k := range vt {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { return fmt.Sprint(keys[i]) < fmt.Sprint(keys[j]) })
		sort.SliceStable(keys, func(i, j int) bool { return yamlKeyLess(keys[i], keys[j]) })
		ms := make(yamlv2.MapSlice, len(keys))
		for i, k := range keys {
			ms[i] = yamlv2.MapItem{Key: k, Value: orderedYAML(vt[k])}
		}
		return ms
	case []interface{}:
		for i := range vt {
			vt[i] = orderedYAML(vt[i])
		}
		return vt
	}
	return v
}

// yamlKeyLess is the string comparison of go-yaml v2's keyList.Less (gopkg.in/yaml.v2
// sorter.go, Apache License 2.0). JSON object keys are always strings.
func yamlKeyLess(x, y interface{}) bool {
	a, aok := x.(string)
	b, bok := y.(string)
	if !aok || !bok {
		return fmt.Sprint(x) < fmt.Sprint(y)
	}
	ar, br := []rune(a), []rune(b)
	for i := 0; i < len(ar) && i < len(br); i++ {
		if ar[i] == br[i] {
			continue
		}
		al := unicode.IsLetter(ar[i])
		bl := unicode.IsLetter(br[i])
		if al && bl {
			return ar[i] < br[i]
		}
		if al || bl {
			return bl
		}
		var ai, bi int
		var an, bn int64
		if ar[i] == '0' || br[i] == '0' {
			for j := i - 1; j >= 0 && unicode.IsDigit(ar[j]); j-- {
				if ar[j] != '0' {
					an = 1
					bn = 1
					break
				}
			}
		}
		for ai = i; ai < len(ar) && unicode.IsDigit(ar[ai]); ai++ {
			an = an*10 + int64(ar[ai]-'0')
		}
		for bi = i; bi < len(br) && unicode.IsDigit(br[bi]); bi++ {
			bn = bn*10 + int64(br[bi]-'0')
		}
		if an != bn {
			return an < bn
		}
		if ai != bi {
			return ai < bi
		}
		return ar[i] < br[i]
	}
	return len(ar) < len(br)
}
