# Oct

**A programming language for scientific computing.**

[![CI](https://github.com/yuechen-li-dev/oct/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/yuechen-li-dev/oct/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/yuechen-li-dev/oct?label=release)](https://github.com/yuechen-li-dev/oct/releases)
[![Go version](https://img.shields.io/github/go-mod/go-version/yuechen-li-dev/oct?label=Go&logo=go&logoColor=white)](go.mod)
[![License](https://img.shields.io/github/license/yuechen-li-dev/oct)](LICENSE)

**[Why Oct](#why-oct) · [Quick start](#quick-start) · [Learn Oct](Language/reference/learn/01-first-program.md) · [Language reference](Language/reference/README.md) · [Examples](Examples/README.md) · [Releases](https://github.com/yuechen-li-dev/oct/releases)**

Oct combines checked units, mathematical values, explicit errors, and executable
tests in a statically typed language. Write and test your program in Oct, then
compile it to a standalone native executable through Go.

**The correct way should be the easy way.** Put meaning in types, make failure
explicit, and use the smallest construct that expresses the work.

## Why Oct

Scientific computing keeps making researchers pay for the wrong abstractions.

- **Python** splits the work between a readable model and compiled machinery
  underneath it. Performance lives in extensions; types, units, and many
  correctness checks live in conventions around the code.
- **C++** gives you control of the machine, then charges you in language
  complexity, memory management, templates, and build systems. Too much of the
  work goes into maintaining the machinery around the model.
- **Rust** puts ownership, borrowing, and lifetimes at the center of the
  programming experience. Scientific work needs equations, quantities, arrays,
  and experiments at the center. Those should shape the language itself.
- **Go** gets the implementation foundation right: straightforward code, fast
  builds, native execution, and simple deployment. Its language is built for
  general software; dimensions, tensor algebra, and scientific contracts are
  not its vocabulary.

**Oct puts the scientific program first.** Quantities carry units. Domain
concepts state which values are valid. Matrix algebra has its own operators.
Experiments can express their states, decisions, tests, and published results
in the same typed language. Go supplies the runtime and native compilation
foundation; Oct supplies the scientific vocabulary.

## Quick start

### Install

Install the released CLI with Go:

```sh
go install github.com/yuechen-li-dev/oct/cmd/oct@v1.1.0
oct version
```

Add Go's binary installation directory to your `PATH` if needed. Prebuilt
[Windows and Linux downloads](https://github.com/yuechen-li-dev/oct/releases/latest)
are also available; follow the [installation guide](docs/releases/INSTALL_1_1.md).
Native compilation requires the Go toolchain; the required version is recorded
in [go.mod](go.mod).

### A small experiment, written in Oct

This load monitor turns displacement measurements into forces and ranks the
next action. Save it as `load_monitor.oct`:

```oct
package Main

concept Force = Float<kg*m/s^2>
concept LoadLimit = Force {
    Require(Self > 0.0kg*m/s^2, "a load limit must be positive")
}

enum Action { KeepLoading Inspect(Force) Stop(Force) }

flow LoadMonitor(stiffness: Matrix<Float<kg/s^2>>, limit: LoadLimit)
accepts displacement: Vector<Float<m>> yields Action -> Void {
    state Measure {
        let reaction = stiffness @ displacement
        let axial = Abs(reaction[0])
        let lateral = Abs(reaction[1])

        yield when utility Action {
            case Action.Stop(axial) when axial >= limit score 100
            case Action.Inspect(lateral) when lateral > limit * 0.25 score 50
            else Action.KeepLoading
        }
        goto Measure
    }
}

fn Main() -> String ! Error {
    let stiffness = matrix[
        [1000.0kg/s^2, 0.0kg/s^2]
        [0.0kg/s^2, 2000.0kg/s^2]
    ]
    let monitor = LoadMonitor(stiffness, 100.0kg*m/s^2)
    Step(monitor, vector[0.02m, 0.01m])
    Step(monitor, vector[0.12m, 0.02m])

    return match Yielded(monitor)? {
        case Action.KeepLoading => "keep loading"
        case Action.Inspect(_) => "inspect the fixture"
        case Action.Stop(_) => "stop the test"
    }
}
```

```sh
oct run load_monitor.oct
oct build load_monitor.oct
```

The program reports `stop the test`. More of the model is visible in the
language than a collection of functions over bare floating-point arrays:

- `LoadLimit` carries a positive-force requirement through `Require`.
- `stiffness @ displacement` contracts a matrix with a vector. The units
  compose to force, and the compiler checks that result against the load limit.
- `flow` names a reactive computation. Each `Step` supplies a typed measurement;
  `yield` reports a decision and keeps the continuation for the next turn.
- `when utility` lists eligible actions and their priorities. When stopping
  and inspection both apply, stopping wins explicitly; the chosen action
  carries the force that caused it.

`oct run` interprets the source. `oct build` writes a standalone native
executable and reports its path. The same model runs through both paths.

### Learn the language

Start with **[Your first Oct program](Language/reference/learn/01-first-program.md)**.
The revised learning path assumes you already know how to program and introduces
Oct's differences through complete, runnable examples:

| Lesson | What you will learn |
| --- | --- |
| [1. Your first program](Language/reference/learn/01-first-program.md) | Functions, bindings, loops, and tests |
| [2. Values with meaning](Language/reference/learn/02-domain-values.md) | Domain types, units, immutable updates, and errors |
| [3. Functions and alternatives](Language/reference/learn/03-functions-and-alternatives.md) | Callbacks, explicit captures, enums, and options |
| [4. Collections and mathematics](Language/reference/learn/04-collections.md) | Arrays, masks, vectors, and matrices |
| [5. Choosing Oct's unusual features](Language/reference/learn/05-choosing-features.md) | Templates, queries, utility decisions, FLOW, async, and batch |

Keep the [reference](Language/reference/README.md) open for exact rules and the
[glossary](Language/reference/glossary.md) for unfamiliar names. The
[library directory](Language/reference/libraries.md) and
[science guide](docs/science/README.md) help you find the right package.

## What you can express

| Feature | In Oct |
| --- | --- |
| Scientific quantities | [Units and dimensions](Language/reference/language/units.md) checked through arithmetic, arguments, and results |
| Mathematical values | [Vectors, matrices, Einstein notation, and differential operators](Language/reference/language/vectors-matrices-and-tensors.md) |
| Domain contracts | [Concepts and refinements](Language/reference/language/concepts-and-refinements.md), nominal records, and immutable `with` updates |
| Typed reuse | [Explicit templates and selectors](Language/reference/language/templates-and-selectors.md), plus [functions with explicit captures](Language/reference/language/function-values.md) |
| Data processing | [Read-only queries](Language/reference/runtime/queries.md) over arrays and record tables; [batch](Language/reference/runtime/batch.md) mapping with ordered results |
| Stateful behavior | [FLOW state machines](Language/reference/runtime/flows-and-state-machines.md), reactive turns, and deterministic [utility decisions](Language/reference/language/utility-selection.md) |
| Verification and output | [Facts and theories](Language/reference/tooling/testing.md), [benchmarks and typed artifacts](Language/reference/tooling/artifacts-and-benchmarks.md), and document generation |

Start with ordinary functions and values. The advanced constructs are there
when your program needs them, with their own explicit rules and boundaries.

## Try the examples from source

```sh
git clone https://github.com/yuechen-li-dev/oct.git
cd oct
go run ./cmd/oct test Language/ReferenceExamples/valid --execution compiled
```

This runs the tutorial/reference examples through native execution. Use
`--execution interpreted` to exercise the interpreter. From a checkout, replace
`oct` with `go run ./cmd/oct` in the commands above.

For a complete application, try the
[Smart Greenhouse Controller](Examples/SmartGreenhouseController/):

```sh
go run ./cmd/oct test Examples/SmartGreenhouseController --execution compiled
```

Explore [more examples](Examples/README.md), then
[create a package](Language/reference/tooling/cli.md#package-scaffolding) when
you are ready to start a project.

## Why the name

Oct began as **Octave Modern**, out of frustration with GNU Octave. Chasing
MATLAB compatibility felt like preserving another tool's decisions instead
of solving the problems that mattered for scientific computing. The goal
became a modern scientific language with its own ideas about types,
mathematics, and experiments. Oct grew into that language, and the shorter
name stuck.

## Project and development

[Oct 1.1.0](docs/releases/OCT_1_1_RELEASE_NOTES.md) builds on the scientific
language and Go-backed native toolchain. The [support guide](Language/reference/support.md)
and [release surface manifest](docs/releases/OCT_1_0_SURFACE_MANIFEST.md) distinguish
supported language features from experimental or separately governed tools,
including Make, OctGen, alternate backends, SDSL-V, and Prometheus.

- [Documentation](docs/README.md) — architecture, tooling, builds, and workflows.
- [Language contracts](Language/README.md) — executable valid and invalid specimens.
- [Reference coverage](Language/reference/coverage.md) — feature ownership and contract links.
- [Testing guide](docs/GO_TEST_LANES.md) — fast, integration, and external-tool lanes.
- [Issues](https://github.com/yuechen-li-dev/oct/issues) — bug reports and discussion.

Contributions should pair language changes with executable contracts and keep
the reference current. See [AGENTS.md](AGENTS.md) for repository conventions.
Concept Vulkan development lives in the [Concept repository](https://github.com/yuechen-li-dev/Concept).

## License

Oct is distributed under the [GNU General Public License v3](LICENSE).
