# kubectl-neat

Remove clutter from Kubernetes manifests to make them more readable.

> **This is a maintained fork of [itaysk/kubectl-neat](https://github.com/itaysk/kubectl-neat)**,
> which its author, [Itay Shakury](https://github.com/itaysk), declared feature-complete and
> unmaintained in 2025 and invited people to fork. Thank you, Itay. Changes since the fork are
> listed under [What's new in this fork](#whats-new-in-this-fork); ideas taken from other forks
> and unmerged upstream pull requests are credited in [Credits](#credits).


## Demo

Here is a result of a `kubectl get pod -o yaml` for a simple Pod. The lines marked in red are considered redundant and will be removed from the output by kubectl-neat.

![demo](./demo.png)

## Why

When you create a Kubernetes resource, let's say a Pod, Kubernetes adds a whole bunch of internal system information to the yaml or json that you originally authored. This includes:

- Metadata such as creation timestamp, or some internal IDs
- Fill in for missing attributes with default values
- Additional system attributes created by admission controllers, such as service account token
- Status information

If you try to `kubectl get` resources you have created, they will no longer look like what you originally authored, and will be unreadably verbose.   
`kubectl-neat` cleans up that redundant information for you.

## Installation

Build from source (Go 1.26+):

```bash
git clone https://github.com/ayates83/kubectl-neat.git
cd kubectl-neat
make build            # binary in dist/
```

or download a binary for Linux, macOS or Windows from
[Releases](https://github.com/ayates83/kubectl-neat/releases). Each release also carries a krew
manifest (substitute the release you want):

```bash
kubectl krew install --manifest-url=https://github.com/ayates83/kubectl-neat/releases/download/v3.0.0/neat.yaml
```

On macOS the binaries are not notarized; if Gatekeeper blocks one, run
`xattr -d com.apple.quarantine ./kubectl-neat`.

`go install` does not work: kubectl-neat uses the defaulting code in `k8s.io/kubernetes`,
which can only be imported with `replace` directives, and `go install` ignores them.

`kubectl krew install neat` installs the original upstream plugin (`v2.x`), not this fork (`v3.x`).
`kubectl neat version` tells them apart.

When used as a kubectl plugin the command is `kubectl neat`, and when used as a standalone
executable it's `kubectl-neat`.

## Usage

There are two modes of operation that specify where to get the input document from: a local file or from  Kubernetes.

### Local - file or Stdin

This is the default mode if you run just `kubectl neat`. This command accepts an optional flag `-f/--file` which specifies the file to neat. It can be a path to a local file, or `-` to read the file from stdin. If omitted, it will default to `-`. The file must be a yaml or json file and a valid Kubernetes resource.

There's another optional optional flag, `-o/--output` which specifies the format for the output. If omitted it will default to the same format of the input (auto-detected).

Examples:
```bash
kubectl get pod mypod -o yaml | kubectl neat

kubectl get pod mypod -oyaml | kubectl neat -o json

kubectl neat -f - <./my-pod.json

kubectl neat -f ./my-pod.json

kubectl neat -f ./my-pod.json --output yaml
```

### Kubernetes - kubectl get wrapper

This mode is invoked by calling the `get` subcommand, i.e `kubectl neat get ...`. It is a convenience to run `kubectl get` and then `kubectl neat` the output in a single command. It accepts any argument that `kubectl get` accepts and passes those arguments as is to `kubectl get`. Since it executes `kubectl`, it need to be able to find it in the path.

Examples:
```bash
kubectl neat get -- pod mypod -oyaml
kubectl neat get -- svc -n default myservice --output json
```

### Several documents at once

YAML input may hold several documents separated by `---`. Each is neated, and the result is
written back as a YAML stream, or as a `v1 List` when `-o json` is given.

```bash
kubectl neat -f ./exported-namespace.yaml > clean.yaml
```

### Removing your own metadata

Tooling of your own leaves annotations and labels behind too. Strip them with repeatable,
comma-separable flags, or set the equivalent environment variables once. A trailing `*`
matches a prefix.

```bash
kubectl get deploy web -o yaml | kubectl neat --strip-annotation 'example.com/*' --strip-label team

export KUBECTL_NEAT_STRIP_ANNOTATIONS='example.com/*,ticket'
export KUBECTL_NEAT_STRIP_LABELS='team'
```

A label that a controller's selector requires is never removed from its pod template, since
the result would be rejected by the API server.

### Diff - de-clutter `kubectl diff`

```bash
export KUBECTL_EXTERNAL_DIFF="kubectl-neat --diff"
kubectl diff -f deploy.yaml
```

Both sides are neated before they are compared, so the diff shows what you changed instead of
`resourceVersion`, `generation` and `managedFields` churn. It is a flag and not a subcommand
because kubectl appends its two directories after the command it is given.

To compare two files or directories directly:

```bash
kubectl neat diff live.yaml desired.yaml
```

The inputs are never modified. The diff program is `diff -u -N` unless `KUBECTL_NEAT_DIFF`
names another (for example `dyff between`; on Windows, where there is no `diff`, this is
required). The exit status is diff's: `0` no differences, `1` differences, `2` an error.

# How it works

Besides general tidying for status, metadata, and empty fields, kubectl-neat primarily looks for two types of things: default values inserted by Kubernetes' object model, and common mutating controllers.

## Kubernetes object model defaults

For de-defaulting Kubernetes' object model, we invoke the same code that Kubernetes would have, and see what default values were assigned. If these observed values look like the ones we have in the incoming spec, we conclude they are default. If they weren't, and the user manually set a field to it's default value, it's not a bad thing to remove it anyway.

## Common mutating controllers

Here are the [recommended](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#what-does-each-admission-controller-do) admission controllers, and their relation to kubectl-neat:

controller | description | neat
---|---|---
NamespaceLifecycle | rejects operations on resources in namespaces being deleted | ignore
LimitRanger | set default values for resource requests and limits | ignore
ServiceAccount | set default service account and assign token | Remove `kube-api-access-*` and `default-token-*` volumes and their mounts in every container type, `serviceAccountName: default`, and deprecated `spec.serviceAccount`
TaintNodesByCondition | automatically taint a node based on node conditions | ignore (acts on Nodes)
Priority | validate priority class and add it's value | Remove `spec.priority` and `spec.preemptionPolicy` from Pods: they are always computed from `priorityClassName`
DefaultTolerationSeconds | configure pods to temporarily tolarate notready and unreachable taints | Remove the two 300-second `NoExecute` tolerations it adds; tolerations with any other value are kept
DefaultStorageClass | validate and set default storage class for new pvc | ignore
StorageObjectInUseProtection | prevent deletion of pvc/pv in use by adding a finalizer | ignore
PersistentVolumeClaimResize | enforce pvc resizing only for enabled storage classes | ignore
MutatingAdmissionWebhook | implement the mutating webhook feature | ignore
ValidatingAdmissionWebhook | implement the validating webhook feature | ignore
RuntimeClass | add pod overhead according to runtime class | TODO
ResourceQuota | implement the resource qouta feature | ignore
Kubernetes Scheduler | assign pods to nodes | Remove `spec.nodeName`

## Controllers, platforms and tools

Beyond admission, kubectl-neat removes fields that other parts of a cluster write. Every key is
one the component sets itself, and it is only removed if the cluster regenerates it or it is
pure history. (`pv.kubernetes.io/provisioned-by` fails that test and is kept: without it a CSI
provisioner will not delete the volume behind a `reclaimPolicy: Delete` PV.) The full list is in
[`cmd/server.go`](cmd/server.go).

source | removed
---|---
Deployments, DaemonSets | `deployment.kubernetes.io/*` revision annotations, `deprecated.daemonset.template.generation`
kubectl | `last-applied-configuration`, `restartedAt` (from `rollout restart`), `kubernetes.io/change-cause`
Pod controllers | `pod-template-hash`, `controller-revision-hash`, StatefulSet pod name and index labels (on Pods only)
Jobs | the generated `spec.selector` and `controller-uid` / `job-name` labels, unless `manualSelector: true`. Left in place, the Job cannot be re-applied
Services | allocated `clusterIP` / `clusterIPs` (not `None`: headless is authored), default single-stack IPv4 `ipFamilies` / `ipFamilyPolicy`
PersistentVolumeClaims | binder annotations; `volumeName` only when the binder chose it
Namespaces | the `kubernetes.io/metadata.name` label and the `kubernetes` finalizer
All templates | `creationTimestamp: null`
kubelet, CNI | static-pod `kubernetes.io/config.*`; Multus, OVN-Kubernetes and Calico pod-network annotations
OpenShift | `openshift.io/scc` and the validated subject type; per-cluster `sa.scc` UID, group and MCS ranges; on Pods admitted through an SCC, the `runAsUser`, `fsGroup` and SELinux level it injected from that namespace's allocation (restricted-v2 rejects them in any other namespace); `openshift.io/requester`; the generated `<sa>-dockercfg-*` pull secret on Pods; on Routes, a generated `spec.host` and the Route API defaults
GitOps | Argo CD `tracking-id`, Flux `kustomize.toolkit.fluxcd.io/*` and `helm.toolkit.fluxcd.io/*` labels, kustomize `config.kubernetes.io/origin`

### Known limitations

- **Allocated `nodePort`s are kept.** Without `managedFields` (which `kubectl get` omits) a port the
  cluster picked cannot be told from one that was pinned, and dropping a pinned port breaks its
  clients. Re-creating a NodePort Service in the same cluster fails until you remove it.
- **Values you set to their default are removed too.** The result is equivalent on apply, but
  the output no longer shows that you chose it.

## What's new in this fork

- **Defaults removed for every built-in API group.** Upstream only registered core/v1 defaulting,
  so Deployments, StatefulSets, DaemonSets, Jobs, CronJobs, HPAs and the rest kept every server
  default: `progressDeadlineSeconds`, `revisionHistoryLimit`, the rolling-update block, and all
  pod-template defaults. A Deployment made by `kubectl create deployment` neats to 19 lines;
  upstream leaves 37.
- **Cluster-written fields** from admission, controllers, OpenShift and GitOps tools, as above
  (upstream issues #12, #64, #74).
- **Multi-document YAML** (#109).
- **`--strip-annotation` / `--strip-label`** for site-specific metadata.
- **Diff mode** for `kubectl diff`.
- **Windows builds** (#114).
- **Fixes**: `neat get` failed whenever kubectl printed a warning, and mis-detected `-o json`;
  invalid input shorter than 20 characters panicked.
- Kubernetes 1.36 libraries; Go 1.26.

## Credits

This fork builds on [Itay Shakury](https://github.com/itaysk)'s kubectl-neat and its contributors.
It also draws on work people did in their own forks and pull requests:

- [fambelic](https://github.com/fambelic) - multi-document YAML, upstream PR #113
- [addreas](https://github.com/addreas) and [meier-christoph](https://github.com/meier-christoph) - neating both sides of `kubectl diff`
- [parinapatel](https://github.com/parinapatel) - user-defined labels to strip
- [jadunham1](https://github.com/jadunham1) - stripping `kubernetes.io/change-cause`
- [flanksource](https://github.com/flanksource) - stripping kustomize and Flux metadata
- [knorr3](https://github.com/knorr3) - OpenShift-specific neating, upstream PR #107
- [varkrish](https://github.com/varkrish) - Windows builds, upstream PR #88
- [davidaparicio](https://github.com/davidaparicio), [erikgb](https://github.com/erikgb), [alexkruc](https://github.com/alexkruc) and others - dependency updates

The implementations here are new, but those ideas and bug reports came first.
