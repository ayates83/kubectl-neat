# AGENTS.md

Guidance for AI coding agents, and humans, working in this repository. See the
[Readme](Readme.md) for what the tool does from a user's point of view.

## What this is

`kubectl-neat` removes clutter from Kubernetes manifests: server defaults, fields the
cluster writes, status, and metadata noise. What is left is close to what a person
authored. This is a maintained fork of `itaysk/kubectl-neat`. The module path is
`github.com/ayates83/kubectl-neat`.

**The one rule that matters:** removing an authored field is a bug that silently changes
what gets applied. Leaving noise in is only cosmetic. When in doubt, keep the field.

## Layout

```text
main.go                      calls cmd.Execute
cmd/cmd.go                   cobra root and `get`; NeatYAMLOrJSON (format detection, multi-doc)
cmd/neat.go                  Neat(): the pipeline; metadata, status, empty-field cleanup
cmd/server.go                fields written by the cluster, not the author (admission,
                             controllers, OpenShift, CNI, GitOps) + --strip-* matching
cmd/diff.go                  `diff` subcommand and the --diff flag for KUBECTL_EXTERNAL_DIFF
pkg/defaults/defaults.go     removes values equal to the API server's defaulting output
pkg/testutil/                JSONEqual
test/fixtures/<name>-raw.*   input; <name>-neat.json is the expected output (TestNeat)
test/*.bats                  end-to-end CLI, kubectl and krew tests; test/kubectl-stub fakes kubectl
hack/update-kubernetes-deps.sh   the only supported way to bump Kubernetes
```

## Commands

```bash
make build                      # dist/kubectl-neat_<os>_<arch>
make test-unit                  # go test -v ./...
test -z "$(gofmt -s -d .)"      # CI fails on any gofmt diff
go vet ./...                    # CI runs this too
bats test/e2e-cli.bats          # needs `make build` first; bats-core can be run from a clone
./hack/update-kubernetes-deps.sh v1.36.4 && go mod tidy
```

Go version comes from `go.mod`. `goreleaser` v2 may need a newer Go than the project
does: `GOTOOLCHAIN=auto goreleaser release --snapshot --clean --skip=publish`.

## Rules for changing what gets removed

1. **A field is removed only if the cluster regenerates it, or it is pure history.**
   Being set by a controller is not enough. Counter-example, kept on purpose:
   `pv.kubernetes.io/provisioned-by`. Nothing re-adds it, and without it a CSI provisioner
   will not delete the volume behind a `reclaimPolicy: Delete` PV.
2. **Every key in `cmd/server.go` must be a named constant in the source of the component
   that sets it.** Cite the source in the PR or commit. Don't add a key from memory or from
   one cluster's output, and never add a key that people also set by hand.
3. **Scope rules to the kind that needs them, and check the conditions that make a field
   authored.** Examples already in the code:
   - `clusterIP: None` (headless) is kept
   - a Job with `manualSelector: true` keeps its selector
   - `pod-template-hash` is stripped from Pods but not ReplicaSets, where it is in the selector
   - a Route keeps `spec.host` unless `openshift.io/host.generated: "true"`
   - user-supplied `--strip-label` never removes a label a selector requires
4. **Every removal needs a negative test beside it**, a case where the same field must
   survive. Put it in the table in `cmd/server_test.go`. Then break the rule on purpose and
   confirm the negative test fails. A new test that passes first time proves nothing until
   you have seen it fail.
5. **Default removal is generic.** `pkg/defaults` compares against the defaulting functions
   registered in `schemeAdders`. To cover a new built-in API group, register its
   `k8s.io/kubernetes/pkg/apis/<group>/<version>` package there. Do not hand-write default
   values that the scheme already knows.

## Gotchas

- **`go install` does not work, by design.** Depending on `k8s.io/kubernetes` needs
  `replace` directives, which `go install` ignores. Don't "fix" this by deleting the
  replaces. Vendoring a copy of the defaulting code has also been tried in other forks,
  and it goes stale.
- **cobra flag state outlives `Execute()`** inside one test binary. A test that runs
  `rootCmd` must reset any flag an earlier test set (see `TestGetIgnoresKubectlWarnings`).
- **`KUBECTL_EXTERNAL_DIFF` appends kubectl's two directories after the words you give
  it**, and drops words outside `[a-zA-Z0-9-=]`. That's why diff mode is the `--diff` flag
  and not only a subcommand.
- **Diff mode exit codes follow `diff`**: 0 no differences, 1 differences, 2 trouble.
  kubectl reports exit 1 as "differences found", so an internal failure must never exit 1.
- **Diff mode must never modify its inputs.** It neats copies in a temp directory.
- **`neat get` parses kubectl's stdout only.** Warnings arrive on stderr and must not be
  merged into the JSON.
- **Multi-document YAML is split with apimachinery's `YAMLReader`**, the reader kubectl
  uses. Don't replace it with string splitting on `---`.
- **gjson/sjson paths need `escapeKey`** for label and annotation keys, which contain `.` and `/`.

## Fixtures and data: this is a public repository

- Test data is synthetic. Use `example.com` and `.test` domains, zeroed UIDs, documentation
  or clearly fake IPs, and generic names (`web`, `demo`).
- **Never paste output from a real cluster** into fixtures, tests, issues or commit messages.
  Hostnames, namespaces, registry paths, UIDs, node names, usernames in
  `openshift.io/requester`, and SCC UID ranges all identify an environment. If a real object
  shaped the case, rebuild it by hand.
- No credentials, tokens, kubeconfigs or pull secrets, even expired ones.
  `test/fixtures/secret1-raw.yaml` holds deliberately fake `user`/`pass` data from upstream.

## Commits and pull requests

- Small, single-purpose commits. The subject says what changed; the body says why, and
  cites the Kubernetes/OpenShift source for any new rule.
- Reference upstream issue numbers (`#109`) where they apply. Credit ideas taken from
  other forks or upstream pull requests in the commit and in the Readme's Credits section,
  and don't copy code with incompatible authorship.
- Before pushing: `gofmt`, `go vet`, `make test-unit`, and `bats test/e2e-cli.bats` if the
  CLI changed.
- Update the Readme when behaviour a user can see changes.
