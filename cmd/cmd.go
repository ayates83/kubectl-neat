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
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"unicode"

	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"
	"github.com/tidwall/sjson"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
)

var outputFormat *string
var inputFile *string

func init() {
	outputFormat = rootCmd.PersistentFlags().StringP("output", "o", "yaml", "output format: yaml or json")
	inputFile = rootCmd.Flags().StringP("file", "f", "-", "file path to neat, or - to read from stdin")
	rootCmd.SetOut(os.Stdout)
	rootCmd.SetErr(os.Stderr)
	rootCmd.MarkFlagFilename("file")
	rootCmd.AddCommand(getCmd)
	rootCmd.AddCommand(versionCmd)
}

// Execute is the entry point for the command package
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:          "kubectl-neat",
	SilenceUsage: true, // usage is for flag mistakes, not for bad input
	Example: `kubectl get pod mypod -o yaml | kubectl neat
kubectl get pod mypod -oyaml | kubectl neat -o json
kubectl neat -f - <./my-pod.json
kubectl neat -f ./my-pod.json
kubectl neat -f ./my-pod.json --output yaml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var in, out []byte
		var err error
		if *inputFile == "-" {
			stdin := cmd.InOrStdin()
			in, err = io.ReadAll(stdin)
			if err != nil {
				return err
			}
		} else {
			in, err = os.ReadFile(*inputFile)
			if err != nil {
				return err
			}
		}
		outFormat := *outputFormat
		if !cmd.Flag("output").Changed {
			outFormat = "same"
		}
		out, err = NeatYAMLOrJSON(in, outFormat)
		if err != nil {
			return err
		}
		cmd.Print(string(out))
		return nil
	},
}

var kubectl string = "kubectl"

var getCmd = &cobra.Command{
	Use: "get",
	Example: `kubectl neat get -- pod mypod -oyaml
kubectl neat get -- svc -n default myservice --output json`,
	FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true}, //don't try to validate kubectl get's flags
	RunE: func(cmd *cobra.Command, args []string) error {
		var out []byte
		var err error
		//reset defaults
		//there are two output settings in this subcommand: kubectl get's and kubectl-neat's
		//any combination of those can be provided by using the output flag in either side of the --
		//the most efficient is kubectl: json, kubectl-neat: yaml
		//0--0->Y--J #choose what's best for us
		//0--Y->Y--Y #user did specify output in kubectl, so respect that
		//0--J->J--J #user did specify output in kubectl, so respect that
		//Y--0->Y--J #user doesn't care about kubectl so use json but convert back
		//J--0->J--J #user expects json so use it for foth
		//if the user specified both side we can't touch it

		//the desired kubectl get output is always json, unless it was explicitly set by the user to yaml in which case the arg is overriden when concatenating the args later
		cmdArgs := append([]string{"get", "-o", "json"}, args...)
		kubectlCmd := exec.Command(kubectl, cmdArgs...)
		kres, err := kubectlCmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("Error invoking kubectl as %v %v", kubectlCmd.Args, err)
		}
		//handle the case of 0--J->J--J
		outFormat := *outputFormat
		kubeout := "yaml"
		for _, arg := range args {
			if arg == "json" || arg == "ojson" {
				outFormat = "json"
			}
		}
		if !cmd.Flag("output").Changed && kubeout == "json" {
			outFormat = "json"
		}
		out, err = NeatYAMLOrJSON(kres, outFormat)
		if err != nil {
			return err
		}
		cmd.Println(string(out))
		return nil
	},
}

// populated by goreleaser
var (
	Version = "v0.0.0+unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print kubectl-neat version",
	Long:  "Print the version of kubectl-neat",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("kubectl-neat version: %s\n", Version)
	},
}

func isJSON(s []byte) bool {
	return bytes.HasPrefix(bytes.TrimLeftFunc(s, unicode.IsSpace), []byte{'{'})
}

// NeatYAMLOrJSON converts 'in' to json if needed, invokes neat, and converts back if needed according the the outputFormat argument: yaml/json/same.
// YAML input may hold several documents separated by '---' (#109). They are neated one by one and
// written back as a YAML stream, or, when JSON output is requested, as the items of a v1 List.
func NeatYAMLOrJSON(in []byte, outputFormat string) (out []byte, err error) {
	if isJSON(in) {
		outjson, err := Neat(string(in))
		if err != nil {
			return nil, fmt.Errorf("error neating : %v", err)
		}
		if outputFormat == "yaml" {
			return yaml.JSONToYAML([]byte(outjson))
		}
		return []byte(outjson), nil
	}

	docs, err := splitYAMLDocuments(in)
	if err != nil {
		return nil, err
	}
	neated := make([]string, 0, len(docs))
	for i, doc := range docs {
		injson, err := yaml.YAMLToJSON(doc)
		if err != nil {
			return nil, fmt.Errorf("error converting from yaml to json%s : %v", docLabel(i, len(docs)), err)
		}
		if t := bytes.TrimSpace(injson); len(t) == 0 || string(t) == "null" {
			continue // a document holding only comments
		}
		outjson, err := Neat(string(injson))
		if err != nil {
			return nil, fmt.Errorf("error neating%s : %v", docLabel(i, len(docs)), err)
		}
		neated = append(neated, outjson)
	}

	if outputFormat == "json" {
		if len(neated) == 1 {
			return []byte(neated[0]), nil
		}
		list := `{"apiVersion":"v1","kind":"List","items":[]}`
		for i, n := range neated {
			if list, err = sjson.SetRaw(list, fmt.Sprintf("items.%d", i), n); err != nil {
				return nil, fmt.Errorf("error building list : %v", err)
			}
		}
		return []byte(list), nil
	}

	var buf bytes.Buffer
	for i, n := range neated {
		y, err := yaml.JSONToYAML([]byte(n))
		if err != nil {
			return nil, fmt.Errorf("error converting from json to yaml : %v", err)
		}
		if i > 0 {
			buf.WriteString("---\n")
		}
		buf.Write(y)
	}
	return buf.Bytes(), nil
}

// splitYAMLDocuments splits a YAML stream on '---' separator lines. It uses the same reader
// kubectl does, so a separator is only recognised at the start of a line, never inside a
// block scalar, and a leading '---' does not produce an empty document.
func splitYAMLDocuments(in []byte) ([][]byte, error) {
	reader := k8syaml.NewYAMLReader(bufio.NewReader(bytes.NewReader(in)))
	var docs [][]byte
	for {
		doc, err := reader.Read()
		if err == io.EOF {
			return docs, nil
		}
		if err != nil {
			return nil, fmt.Errorf("error reading yaml document %d : %v", len(docs)+1, err)
		}
		docs = append(docs, doc)
	}
}

func docLabel(i, n int) string {
	if n == 1 {
		return ""
	}
	return fmt.Sprintf(" (document %d of %d)", i+1, n)
}
