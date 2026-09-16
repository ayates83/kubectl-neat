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
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

const diffProgramEnv = "KUBECTL_NEAT_DIFF"

var diffMode bool

var diffCmd = &cobra.Command{
	Use:   "diff FROM TO",
	Short: "Diff two manifests or directories after neating both sides",
	Long: `Neat copies of FROM and TO, then diff them. Files that cannot be neated are
compared as they are. The inputs are never modified.

To de-clutter kubectl diff, use the --diff flag form, because kubectl appends its
own arguments after the command it is given:

  export KUBECTL_EXTERNAL_DIFF="kubectl-neat --diff"
  kubectl diff -f deploy.yaml

The diff program defaults to "diff -u -N"; set ` + diffProgramEnv + ` to use another
(for example "dyff between"). The exit code is the diff program's: 0 for no
differences, 1 for differences, anything else for an error.`,
	Example: `kubectl neat diff live.yaml desired.yaml
KUBECTL_EXTERNAL_DIFF="kubectl-neat --diff" kubectl diff -f deploy.yaml`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runNeatDiff(cmd, args[0], args[1])
	},
}

func init() {
	rootCmd.Flags().BoolVar(&diffMode, "diff", false,
		"diff the two paths given as arguments after neating them; for KUBECTL_EXTERNAL_DIFF")
	rootCmd.AddCommand(diffCmd)
}

// exitError carries the diff program's exit code out through cobra.
type exitError struct{ code int }

func (e exitError) Error() string { return fmt.Sprintf("diff exited with status %d", e.code) }

// runNeatDiff reports its own failures with exit status 2, as diff does. Returning them
// as ordinary errors would exit 1, which kubectl diff reads as "differences found".
func runNeatDiff(cmd *cobra.Command, from, to string) error {
	err := neatDiff(cmd, from, to)
	var exit exitError
	if err == nil || errors.As(err, &exit) {
		return err
	}
	fmt.Fprintln(cmd.ErrOrStderr(), "Error:", err)
	return exitError{2}
}

func neatDiff(cmd *cobra.Command, from, to string) error {
	work, err := os.MkdirTemp("", "kubectl-neat-diff-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)

	// Keep each side's base name so the diff headers read as they would without neat,
	// e.g. LIVE-123/apps.v1.Deployment.default.web. Identical base names get a prefix.
	fromName, toName := filepath.Base(from), filepath.Base(to)
	if fromName == toName {
		fromName, toName = "a-"+fromName, "b-"+toName
	}
	for _, side := range []struct{ src, dst string }{{from, fromName}, {to, toName}} {
		if err := copyNeated(cmd.ErrOrStderr(), side.src, filepath.Join(work, side.dst)); err != nil {
			return err
		}
	}

	program := []string{"diff", "-u", "-N"}
	if custom := strings.Fields(os.Getenv(diffProgramEnv)); len(custom) > 0 {
		program = custom
	}
	c := exec.Command(program[0], append(program[1:], fromName, toName)...)
	c.Dir = work
	c.Stdout = cmd.OutOrStdout()
	c.Stderr = cmd.ErrOrStderr()
	err = c.Run()
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return exitError{ee.ExitCode()}
	}
	return err
}

// copyNeated copies a file, or every regular file under a directory, neating each one.
func copyNeated(warn io.Writer, src, dst string) error {
	info, err := os.Stat(src)
	if os.IsNotExist(err) {
		return nil // diff -N treats a missing side as empty
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return neatFile(warn, src, dst)
	}
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		return neatFile(warn, path, target)
	})
}

func neatFile(warn io.Writer, src, dst string) error {
	in, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	out, err := NeatYAMLOrJSON(in, "same")
	if err != nil {
		fmt.Fprintf(warn, "kubectl-neat: comparing %s as-is: %v\n", src, err)
		out = in
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	return os.WriteFile(dst, out, 0o600)
}
