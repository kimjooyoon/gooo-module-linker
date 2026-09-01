package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/kimjooyoon/gooo-module-linker/internal/linker"
)

func main() {
	policyPath := flag.String("policy", "", "path to authoritative .gooo policy")
	outIR := flag.String("out-ir", "", "path for linked IR JSON")
	outGo := flag.String("out-go", "", "path for generated Go artifact")
	expect := flag.String("expect", "", "expected judgment status")
	flag.Parse()
	if *policyPath == "" || flag.NArg() == 0 {
		fail("-policy and at least one .gooo module input are required")
	}
	policy, err := linker.ParsePolicyFile(*policyPath)
	if err != nil {
		fail(err.Error())
	}
	modules := make([]linker.Module, 0, flag.NArg())
	for _, path := range flag.Args() {
		module, err := linker.ParseModuleFile(path)
		if err != nil {
			fail(err.Error())
		}
		modules = append(modules, module)
	}
	graph, err := linker.Link(policy, modules)
	if err != nil {
		fail(err.Error())
	}
	if *outIR != "" {
		if err := linker.WriteGraph(*outIR, graph); err != nil {
			fail(err.Error())
		}
	}
	if *outGo != "" {
		generated, err := linker.EmitGo(graph, policy.Generation)
		if err != nil {
			fail(err.Error())
		}
		if err := linker.WriteBytes(*outGo, generated); err != nil {
			fail(err.Error())
		}
	}
	if *expect != "" && graph.Status != *expect {
		fail(fmt.Sprintf("expected %s, got %s", *expect, graph.Status))
	}
	fmt.Printf("status=%s canonical_digest=%s modules=%d edges=%d diagnostics=%d\n", graph.Status, graph.CanonicalDigest, len(graph.Modules), len(graph.Edges), len(graph.Diagnostics))
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}
