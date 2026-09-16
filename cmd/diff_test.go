package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestNeatDiff(t *testing.T) {
	dir := t.TempDir()
	live, merged := filepath.Join(dir, "LIVE-1"), filepath.Join(dir, "MERGED-2")
	for _, d := range []string{live, merged} {
		if err := os.Mkdir(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	liveDoc := `{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"c","uid":"1","resourceVersion":"10"},"data":{"k":"old"}}`
	mergedDoc := `{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"c","uid":"1","resourceVersion":"11"},"data":{"k":"new"}}`
	name := "v1.ConfigMap.default.c"
	if err := os.WriteFile(filepath.Join(live, name), []byte(liveDoc), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(merged, name), []byte(mergedDoc), 0o600); err != nil {
		t.Fatal(err)
	}

	run := func(from, to string) (string, error) {
		var out bytes.Buffer
		c := &cobra.Command{}
		c.SetOut(&out)
		c.SetErr(&out)
		err := runNeatDiff(c, from, to)
		return out.String(), err
	}
	code := func(err error) int {
		var e exitError
		if errors.As(err, &e) {
			return e.code
		}
		if err != nil {
			return -1
		}
		return 0
	}

	out, err := run(live, merged)
	if code(err) != 1 {
		t.Fatalf("want exit 1 for differences, have %v\n%s", err, out)
	}
	if strings.Contains(out, "resourceVersion") || !strings.Contains(out, `"old"`) || !strings.Contains(out, `"new"`) {
		t.Errorf("want only the data change, neated:\n%s", out)
	}
	if !strings.Contains(out, "LIVE-1/"+name) {
		t.Errorf("diff headers should keep kubectl's directory names:\n%s", out)
	}
	if b, _ := os.ReadFile(filepath.Join(live, name)); string(b) != liveDoc {
		t.Errorf("input was modified")
	}

	if _, err = run(live, live); code(err) != 0 {
		t.Errorf("want exit 0 for identical sides, have %v", err)
	}

	t.Setenv(diffProgramEnv, "kubectl-neat-no-such-diff-program")
	if out, err = run(live, merged); code(err) != 2 {
		t.Errorf("a broken diff program must exit 2, not 1 (which kubectl reads as differences); have %v\n%s", err, out)
	}
}
