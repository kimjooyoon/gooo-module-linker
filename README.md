# gooo-module-linker

`gooo-module-linker` is a minimal module system for Gooo. A `.gooo` module
declares its identity, exact release, immutable digest, exports, imports, and
symbol ownership. The linker turns any input order into one canonical linked
IR and can emit a buildable Go artifact.

The semantic authority is `meta/module-linker.gooo`. Go code only parses the
declarations, executes the plan, and emits the linked artifact. The policy
declares the judgment precedence `REFUTED > UNKNOWN > CLOSED`, the six-field
UNKNOWN evidence contract, cycle policies, and the Go generation plan.

## Module syntax

```text
module "example/core" release "v1.0.0" digest "sha256:<64 lowercase hex>" cycle_policy "acyclic" {
  export "CoreValue" owner "example/core"
}
```

Imports require the exact release identity and the target's immutable digest:

```text
import "example/core" release "v1.0.0" digest "sha256:<target digest>" symbol "CoreValue" owner "example/core"
```

The declared module digest is the SHA-256 digest of its canonical semantic
declaration. Dependency digest fields are excluded from that self-digest so
cycles can still be represented without a circular hash.

## Judgments

- Duplicate exports, ownership violations, exact-release mismatches, and
  digest mismatches are `REFUTED`.
- Missing immutable releases or requested symbols are `UNKNOWN` and preserve
  `stage`, `step`, `reason`, `unknown_class`, `next_operation`, and
  `blocked_by`.
- An acyclic cycle policy is `REFUTED`; an explicit allow policy is `CLOSED`;
  an indeterminate policy is `UNKNOWN`.

## Usage

```sh
go run ./cmd/gooo-module-linker \
  -policy meta/module-linker.gooo \
  -out-ir /tmp/linked.ir.json \
  -out-go /tmp/generated_link.go \
  fixtures/modules/app.gooo fixtures/modules/feature.gooo fixtures/modules/core.gooo
go build /tmp/generated_link.go
```

The conformance denominator is also metacode:

```sh
go run ./cmd/gooo-conformance \
  -policy meta/module-linker.gooo \
  -manifest meta/conformance.gooo \
  -root .
```

The authoritative verification path is GitHub Actions on the pull request. It
records exact integer inventory, runtime, generated-artifact, and conformance
metrics in the `gooo-evidence` artifact. The root README is excluded from the
inventory, and all generated output is written to a caller-owned output
directory.

This repository is bootstrapped from the immutable `gooo-repository-bootstrap`
`v0.1.1` release asset. Feature changes after the bootstrap commit are made
through pull requests and verified by GitHub Actions.
