package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kimjooyoon/gooo-module-linker/internal/linker"
)

type result struct {
	Kind               string              `json:"kind"`
	Name               string              `json:"name"`
	Expected           string              `json:"expected"`
	Actual             string              `json:"actual"`
	CanonicalDigest    string              `json:"canonical_digest,omitempty"`
	ComparedDigest     string              `json:"compared_digest,omitempty"`
	Diagnostics        []linker.Diagnostic `json:"diagnostics,omitempty"`
	Passed             bool                `json:"passed"`
	UnknownFieldsValid bool                `json:"unknown_fields_valid"`
	Failure            string              `json:"failure,omitempty"`
}

type report struct {
	Schema     string   `json:"schema"`
	Identity   string   `json:"identity"`
	Precedence []string `json:"precedence"`
	Total      int      `json:"total"`
	Selected   int      `json:"selected"`
	Executed   int      `json:"executed"`
	Reused     int      `json:"reused"`
	Failed     int      `json:"failed"`
	Unknown    int      `json:"unknown"`
	Results    []result `json:"results"`
}

func main() {
	policyPath := flag.String("policy", "", "path to authoritative .gooo policy")
	manifestPath := flag.String("manifest", "", "path to .gooo conformance denominator")
	root := flag.String("root", ".", "fixture root")
	out := flag.String("out", "", "path for conformance JSON")
	flag.Parse()
	if *policyPath == "" || *manifestPath == "" {
		fail("-policy and -manifest are required")
	}
	policy, err := linker.ParsePolicyFile(*policyPath)
	if err != nil {
		fail(err.Error())
	}
	manifest, err := linker.ParseConformanceFile(*manifestPath)
	if err != nil {
		fail(err.Error())
	}
	output := report{Schema: "gooo.conformance/v1", Identity: manifest.Identity, Precedence: policy.Precedence, Total: len(manifest.Scenarios) + len(manifest.Comparisons)}
	for _, scenario := range manifest.Scenarios {
		if !scenario.Selected {
			continue
		}
		output.Selected++
		output.Executed++
		modules, err := loadModules(*root, scenario.Inputs)
		if err != nil {
			output.Failed++
			output.Results = append(output.Results, result{Kind: "scenario", Name: scenario.Name, Expected: scenario.Expected, Passed: false, UnknownFieldsValid: false, Failure: err.Error()})
			continue
		}
		graph, err := linker.Link(policy, modules)
		if err != nil {
			output.Failed++
			output.Results = append(output.Results, result{Kind: "scenario", Name: scenario.Name, Expected: scenario.Expected, Passed: false, UnknownFieldsValid: false, Failure: err.Error()})
			continue
		}
		unknownValid := unknownFieldsValid(graph)
		passed := graph.Status == scenario.Expected && unknownValid
		if graph.Status == "UNKNOWN" {
			output.Unknown++
		}
		if !passed {
			output.Failed++
		}
		output.Results = append(output.Results, result{Kind: "scenario", Name: scenario.Name, Expected: scenario.Expected, Actual: graph.Status, CanonicalDigest: graph.CanonicalDigest, Diagnostics: graph.Diagnostics, Passed: passed, UnknownFieldsValid: unknownValid})
	}
	for _, comparison := range manifest.Comparisons {
		if !comparison.Selected {
			continue
		}
		output.Selected++
		output.Executed++
		left, leftErr := linkInputs(policy, *root, comparison.Left)
		right, rightErr := linkInputs(policy, *root, comparison.Right)
		if leftErr != nil || rightErr != nil {
			failure := strings.TrimSpace(fmt.Sprintf("left=%v right=%v", leftErr, rightErr))
			output.Failed++
			output.Results = append(output.Results, result{Kind: "comparison", Name: comparison.Name, Expected: comparison.Expected, Passed: false, UnknownFieldsValid: false, Failure: failure})
			continue
		}
		unknownValid := unknownFieldsValid(left) && unknownFieldsValid(right)
		passed := left.Status == comparison.Expected && right.Status == comparison.Expected && left.CanonicalDigest == right.CanonicalDigest && unknownValid
		if left.Status == "UNKNOWN" || right.Status == "UNKNOWN" {
			output.Unknown++
		}
		if !passed {
			output.Failed++
		}
		output.Results = append(output.Results, result{Kind: "comparison", Name: comparison.Name, Expected: comparison.Expected, Actual: left.Status, CanonicalDigest: left.CanonicalDigest, ComparedDigest: right.CanonicalDigest, Diagnostics: left.Diagnostics, Passed: passed, UnknownFieldsValid: unknownValid})
	}
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		fail(err.Error())
	}
	data = append(data, '\n')
	if *out != "" {
		if err := linker.WriteBytes(*out, data); err != nil {
			fail(err.Error())
		}
	}
	fmt.Printf("total=%d selected=%d executed=%d reused=%d failed=%d unknown=%d\n", output.Total, output.Selected, output.Executed, output.Reused, output.Failed, output.Unknown)
	if output.Failed != 0 {
		os.Exit(1)
	}
}

func linkInputs(policy linker.Policy, root string, inputs []string) (linker.LinkedGraph, error) {
	modules, err := loadModules(root, inputs)
	if err != nil {
		return linker.LinkedGraph{}, err
	}
	return linker.Link(policy, modules)
}

func loadModules(root string, inputs []string) ([]linker.Module, error) {
	modules := make([]linker.Module, 0, len(inputs))
	for _, input := range inputs {
		path := input
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, path)
		}
		module, err := linker.ParseModuleFile(path)
		if err != nil {
			return nil, err
		}
		modules = append(modules, module)
	}
	return modules, nil
}

func unknownFieldsValid(graph linker.LinkedGraph) bool {
	for _, diagnostic := range graph.Diagnostics {
		if diagnostic.Status != "UNKNOWN" || diagnostic.Unknown == nil {
			if diagnostic.Status == "UNKNOWN" {
				return false
			}
			continue
		}
		unknown := diagnostic.Unknown
		if unknown.Stage == "" || unknown.Step == "" || unknown.Reason == "" || unknown.UnknownClass == "" || unknown.NextOperation == "" || unknown.BlockedBy == "" {
			return false
		}
	}
	return true
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}
