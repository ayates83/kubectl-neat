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
	"regexp"
	"slices"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// Every key below is written by the API server, a built-in controller, an admission
// plugin, the kubelet, or a common platform component -- never by a manifest author.
// Each one is a named constant in the source of the component that sets it; keep it
// that way, and do not add keys that people also set by hand.
//
// Being set by a controller is not enough. A key belongs here only if the cluster
// regenerates it, or it is pure history. pv.kubernetes.io/provisioned-by is the
// counter-example: nothing re-adds it, and without it the provisioner refuses to
// delete the backing volume of a reclaimPolicy: Delete PV.

// serverAnnotations are stripped from every metadata block that neat keeps:
// the object itself and any embedded pod/job/claim templates.
var serverAnnotations = []string{
	"kubectl.kubernetes.io/last-applied-configuration",
	"kubectl.kubernetes.io/restartedAt", // kubectl rollout restart
	"kubernetes.io/change-cause",        // kubectl --record (deprecated)

	"deployment.kubernetes.io/revision",
	"deployment.kubernetes.io/revision-history",
	"deployment.kubernetes.io/desired-replicas",
	"deployment.kubernetes.io/max-replicas",
	"deprecated.daemonset.template.generation",
	"endpoints.kubernetes.io/last-change-trigger-time",

	"pv.kubernetes.io/bind-completed",
	"pv.kubernetes.io/bound-by-controller",
	"pv.kubernetes.io/migrated-to",
	"volume.kubernetes.io/selected-node",
	"volume.kubernetes.io/storage-provisioner",
	"volume.beta.kubernetes.io/storage-provisioner",

	"kubernetes.io/config.source", // kubelet, static/mirror pods
	"kubernetes.io/config.seen",
	"kubernetes.io/config.hash",

	"k8s.v1.cni.cncf.io/network-status", // Multus
	"k8s.ovn.org/pod-networks",          // OVN-Kubernetes
	"cni.projectcalico.org/podIP",       // Calico
	"cni.projectcalico.org/podIPs",
	"cni.projectcalico.org/containerID",

	"openshift.io/scc", // OpenShift SCC admission
	"security.openshift.io/validated-scc-subject-type",
	"openshift.io/sa.scc.uid-range", // allocated per cluster, wrong anywhere else
	"openshift.io/sa.scc.supplemental-groups",
	"openshift.io/sa.scc.mcs",
	"openshift.io/requester", // the user who requested the project

	"config.kubernetes.io/origin",    // kustomize build
	"argocd.argoproj.io/tracking-id", // Argo CD
}

// serverLabels are stripped wherever serverAnnotations are.
var serverLabels = []string{
	"kustomize.toolkit.fluxcd.io/name", // Flux
	"kustomize.toolkit.fluxcd.io/namespace",
	"helm.toolkit.fluxcd.io/name",
	"helm.toolkit.fluxcd.io/namespace",
}

// podControllerLabels are stamped onto Pods by their controllers. They are only
// stripped from Pods: on a ReplicaSet, pod-template-hash is also in the selector.
var podControllerLabels = []string{
	"pod-template-hash",
	"controller-revision-hash",
	"statefulset.kubernetes.io/pod-name",
	"apps.kubernetes.io/pod-index",
}

// jobGeneratedLabels are added to a Job's pod template (and copied onto the Job and its
// Pods) together with a generated spec.selector.
var jobGeneratedLabels = []string{
	"controller-uid",
	"batch.kubernetes.io/controller-uid",
	"job-name",
	"batch.kubernetes.io/job-name",
}

// userAnnotations and userLabels are extra keys to strip, from --strip-annotation and
// --strip-label or their environment variables. A trailing '*' matches any suffix.
var userAnnotations, userLabels []string

// selectorKeys returns the label keys a controller's selector requires on the template at
// metadataPath. Removing one of those would make the neated object invalid.
func selectorKeys(in, metadataPath string) []string {
	prefix, ok := strings.CutSuffix(metadataPath, "template.metadata")
	if !ok {
		return nil
	}
	var keys []string
	for _, sel := range []string{prefix + "selector.matchLabels", prefix + "selector"} {
		gjson.Get(in, sel).ForEach(func(k, v gjson.Result) bool {
			if v.Type == gjson.String {
				keys = append(keys, k.Str)
			}
			return true
		})
	}
	return keys
}

// deleteMatching removes every key in the map at path that matches one of patterns,
// except the protected ones.
func deleteMatching(in, path string, patterns []string, protected ...string) (string, error) {
	if len(patterns) == 0 {
		return in, nil
	}
	m := gjson.Get(in, path)
	if !m.IsObject() {
		return in, nil
	}
	var matched []string
	m.ForEach(func(k, _ gjson.Result) bool {
		if slices.Contains(protected, k.Str) {
			return true
		}
		for _, p := range patterns {
			if prefix, ok := strings.CutSuffix(p, "*"); ok && strings.HasPrefix(k.Str, prefix) || k.Str == p {
				matched = append(matched, k.Str)
				break
			}
		}
		return true
	})
	return deleteKeys(in, path, matched)
}

// escapeKey makes a map key usable as a single gjson/sjson path component.
func escapeKey(k string) string {
	r := strings.NewReplacer(`\`, `\\`, `.`, `\.`, `*`, `\*`, `?`, `\?`, `|`, `\|`, `#`, `\#`, `@`, `\@`)
	return r.Replace(k)
}

// deleteKeys removes each key from the map at path, if present.
func deleteKeys(in, path string, keys []string) (string, error) {
	var err error
	m := gjson.Get(in, path)
	if !m.IsObject() {
		return in, nil
	}
	for _, k := range keys {
		if !m.Get(escapeKey(k)).Exists() {
			continue
		}
		in, err = sjson.Delete(in, path+"."+escapeKey(k))
		if err != nil {
			return in, fmt.Errorf("error deleting %s.%s : %v", path, k, err)
		}
	}
	return in, nil
}

// metadataPaths returns every metadata block in the object that survives neating.
func metadataPaths(in string) []string {
	paths := []string{
		"metadata",
		"spec.template.metadata",
		"spec.jobTemplate.metadata",
		"spec.jobTemplate.spec.template.metadata",
	}
	for i := range gjson.Get(in, "spec.volumeClaimTemplates").Array() {
		paths = append(paths, fmt.Sprintf("spec.volumeClaimTemplates.%d.metadata", i))
	}
	return paths
}

// neatServerPopulated removes fields that were filled in by the cluster rather than
// written by whoever authored the manifest. It must run before neatMetadata, because
// some rules read annotations that are about to be removed.
func neatServerPopulated(in string) (string, error) {
	var err error
	kind := gjson.Get(in, "kind").String()
	apiVersion := gjson.Get(in, "apiVersion").String()

	rules := map[string]func(string) (string, error){
		"Pod":                   neatPodAdmission,
		"Job":                   neatJobSelector,
		"Service":               neatServiceAllocation,
		"PersistentVolumeClaim": neatClaimBinding,
		"Namespace":             neatNamespace,
	}
	if rule, ok := rules[kind]; ok {
		if in, err = rule(in); err != nil {
			return in, err
		}
	}
	if kind == "Route" && strings.HasPrefix(apiVersion, "route.openshift.io/") {
		if in, err = neatRoute(in); err != nil {
			return in, err
		}
	}

	for _, p := range metadataPaths(in) {
		if in, err = deleteKeys(in, p+".annotations", serverAnnotations); err != nil {
			return in, err
		}
		if in, err = deleteKeys(in, p+".labels", serverLabels); err != nil {
			return in, err
		}
		if in, err = deleteMatching(in, p+".annotations", userAnnotations); err != nil {
			return in, err
		}
		if in, err = deleteMatching(in, p+".labels", userLabels, selectorKeys(in, p)...); err != nil {
			return in, err
		}
		// A nil metav1.Time serialises as "creationTimestamp": null in every template (#64).
		if c := gjson.Get(in, p+".creationTimestamp"); c.Exists() && c.Type == gjson.Null {
			if in, err = sjson.Delete(in, p+".creationTimestamp"); err != nil {
				return in, err
			}
		}
	}
	return in, nil
}

// neatPodAdmission removes what admission plugins add to every Pod.
func neatPodAdmission(in string) (string, error) {
	var err error
	if in, err = deleteKeys(in, "metadata.labels", podControllerLabels); err != nil {
		return in, err
	}
	if gjson.Get(in, "metadata.labels."+escapeKey("batch.kubernetes.io/controller-uid")).Exists() ||
		gjson.Get(in, "metadata.labels.controller-uid").Exists() {
		if in, err = deleteKeys(in, "metadata.labels", jobGeneratedLabels); err != nil {
			return in, err
		}
	}

	// Priority admission computes both from priorityClassName and rejects a pod that
	// supplies a different value, so neither is ever authored.
	for _, p := range []string{"spec.priority", "spec.preemptionPolicy"} {
		if in, err = sjson.Delete(in, p); err != nil {
			return in, err
		}
	}

	// ServiceAccount admission fills in the default account.
	if gjson.Get(in, "spec.serviceAccountName").String() == "default" {
		if in, err = sjson.Delete(in, "spec.serviceAccountName"); err != nil {
			return in, err
		}
	}

	// DefaultTolerationSeconds admission adds exactly these two (#12).
	tolerations := gjson.Get(in, "spec.tolerations").Array()
	for i := len(tolerations) - 1; i >= 0; i-- {
		t := tolerations[i]
		key := t.Get("key").String()
		if (key == "node.kubernetes.io/not-ready" || key == "node.kubernetes.io/unreachable") &&
			t.Get("operator").String() == "Exists" && t.Get("effect").String() == "NoExecute" &&
			t.Get("tolerationSeconds").Int() == 300 && !t.Get("value").Exists() {
			if in, err = sjson.Delete(in, fmt.Sprintf("spec.tolerations.%d", i)); err != nil {
				return in, err
			}
		}
	}

	if in, err = neatSCCAllocations(in); err != nil {
		return in, err
	}

	// OpenShift links the service account's generated pull secret into every pod.
	sa := gjson.Get(in, "spec.serviceAccountName").String()
	if sa == "" {
		sa = "default"
	}
	generated := regexp.MustCompile("^" + regexp.QuoteMeta(sa) + `-dockercfg-[a-z0-9]{5}$`)
	secrets := gjson.Get(in, "spec.imagePullSecrets").Array()
	for i := len(secrets) - 1; i >= 0; i-- {
		if generated.MatchString(secrets[i].Get("name").String()) {
			if in, err = sjson.Delete(in, fmt.Sprintf("spec.imagePullSecrets.%d", i)); err != nil {
				return in, err
			}
		}
	}

	return neatServiceAccountToken(in)
}

// sccMCSLevel is the shape of the per-namespace SELinux level OpenShift allocates.
var sccMCSLevel = regexp.MustCompile(`^s0:c[0-9]+,c[0-9]+$`)

// isSCCAllocatedID reports whether id looks like the start of an OpenShift namespace UID/GID
// block: the default allocator hands out blocks of 10000 from 1000000000.
func isSCCAllocatedID(r gjson.Result) bool {
	id := r.Int()
	return r.Type == gjson.Number && id >= 1000000000 && id%10000 == 0
}

// neatSCCAllocations removes the user ID, fsGroup and SELinux level that OpenShift's SCC
// admission injects from the namespace's openshift.io/sa.scc.* allocation. They are valid only
// in the namespace they came from: restricted-v2 rejects them anywhere else. Verified against a
// live cluster: the neated pod was forbidden for a non-admin user in a second namespace, and
// accepted with these three removed, when SCC admission re-injected that namespace's values.
// Only pods admitted through an SCC (openshift.io/scc present) are touched.
func neatSCCAllocations(in string) (string, error) {
	var err error
	if !gjson.Get(in, "metadata.annotations."+escapeKey("openshift.io/scc")).Exists() {
		return in, nil
	}
	del := func(path string) {
		if err == nil {
			in, err = sjson.Delete(in, path)
		}
	}
	if isSCCAllocatedID(gjson.Get(in, "spec.securityContext.fsGroup")) {
		del("spec.securityContext.fsGroup")
	}
	if isSCCAllocatedID(gjson.Get(in, "spec.securityContext.runAsUser")) {
		del("spec.securityContext.runAsUser")
	}
	se := gjson.Get(in, "spec.securityContext.seLinuxOptions")
	if se.IsObject() && len(se.Map()) == 1 && sccMCSLevel.MatchString(se.Get("level").String()) {
		del("spec.securityContext.seLinuxOptions")
	}
	for _, cs := range []string{"spec.containers", "spec.initContainers", "spec.ephemeralContainers"} {
		for i, c := range gjson.Get(in, cs).Array() {
			if isSCCAllocatedID(c.Get("securityContext.runAsUser")) {
				del(fmt.Sprintf("%s.%d.securityContext.runAsUser", cs, i))
			}
		}
	}
	return in, err
}

// neatServiceAccountToken removes the projected token volume (kube-api-access-*, since
// Kubernetes 1.22) and the legacy secret volume (default-token-*), with their mounts
// in every kind of container.
func neatServiceAccountToken(in string) (string, error) {
	var err error
	isToken := func(name string) bool {
		return strings.HasPrefix(name, "kube-api-access-") || strings.HasPrefix(name, "default-token-")
	}
	for _, cs := range []string{"spec.containers", "spec.initContainers", "spec.ephemeralContainers"} {
		containers := gjson.Get(in, cs).Array()
		for ci := range containers {
			mounts := containers[ci].Get("volumeMounts").Array()
			for mi := len(mounts) - 1; mi >= 0; mi-- {
				if isToken(mounts[mi].Get("name").String()) {
					if in, err = sjson.Delete(in, fmt.Sprintf("%s.%d.volumeMounts.%d", cs, ci, mi)); err != nil {
						return in, err
					}
				}
			}
		}
	}
	volumes := gjson.Get(in, "spec.volumes").Array()
	for vi := len(volumes) - 1; vi >= 0; vi-- {
		if isToken(volumes[vi].Get("name").String()) {
			if in, err = sjson.Delete(in, fmt.Sprintf("spec.volumes.%d", vi)); err != nil {
				return in, err
			}
		}
	}
	return sjson.Delete(in, "spec.serviceAccount") // deprecated alias of serviceAccountName
}

// neatJobSelector removes the selector and labels the API server generates for a Job.
// Left in place, re-applying the Job fails: a selector without manualSelector is rejected.
func neatJobSelector(in string) (string, error) {
	var err error
	if gjson.Get(in, "spec.manualSelector").Bool() {
		return in, nil
	}
	if in, err = sjson.Delete(in, "spec.selector"); err != nil {
		return in, err
	}
	for _, p := range []string{"spec.template.metadata.labels", "metadata.labels"} {
		if in, err = deleteKeys(in, p, jobGeneratedLabels); err != nil {
			return in, err
		}
	}
	return in, nil
}

// neatServiceAllocation removes cluster-allocated addressing. A headless Service keeps
// clusterIP: None, which is authored and changes behaviour (upstream PR #45 dropped it).
func neatServiceAllocation(in string) (string, error) {
	var err error
	if gjson.Get(in, "spec.clusterIP").String() != "None" {
		for _, p := range []string{"spec.clusterIP", "spec.clusterIPs"} {
			if in, err = sjson.Delete(in, p); err != nil {
				return in, err
			}
		}
	}
	// The server picks single-stack IPv4 when nothing was asked for. Anything else was requested.
	families := gjson.Get(in, "spec.ipFamilies").Array()
	if gjson.Get(in, "spec.ipFamilyPolicy").String() == "SingleStack" &&
		len(families) == 1 && families[0].String() == "IPv4" {
		for _, p := range []string{"spec.ipFamilyPolicy", "spec.ipFamilies"} {
			if in, err = sjson.Delete(in, p); err != nil {
				return in, err
			}
		}
	}
	return in, nil
}

// neatClaimBinding removes a volumeName that the binder chose. A statically requested
// volume has no bound-by-controller annotation and is kept.
func neatClaimBinding(in string) (string, error) {
	if gjson.Get(in, "metadata.annotations."+escapeKey("pv.kubernetes.io/bound-by-controller")).String() == "yes" {
		return sjson.Delete(in, "spec.volumeName")
	}
	return in, nil
}

// neatNamespace removes the name label and finalizer that every Namespace gets.
func neatNamespace(in string) (string, error) {
	var err error
	if in, err = deleteKeys(in, "metadata.labels", []string{"kubernetes.io/metadata.name"}); err != nil {
		return in, err
	}
	f := gjson.Get(in, "spec.finalizers").Array()
	if len(f) == 1 && f[0].String() == "kubernetes" {
		return sjson.Delete(in, "spec.finalizers")
	}
	return in, nil
}

// neatRoute removes a generated host and the Route API defaults. Keeping a generated
// host pins the route to the source cluster's apps domain.
func neatRoute(in string) (string, error) {
	var err error
	hostGenerated := "metadata.annotations." + escapeKey("openshift.io/host.generated")
	if gjson.Get(in, hostGenerated).String() == "true" {
		for _, p := range []string{"spec.host", hostGenerated} {
			if in, err = sjson.Delete(in, p); err != nil {
				return in, err
			}
		}
	}
	defaults := map[string]string{"spec.to.kind": "Service", "spec.to.weight": "100", "spec.wildcardPolicy": "None"}
	for p, v := range defaults {
		if r := gjson.Get(in, p); r.Exists() && r.String() == v {
			if in, err = sjson.Delete(in, p); err != nil {
				return in, err
			}
		}
	}
	return in, nil
}
