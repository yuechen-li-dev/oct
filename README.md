# Oct

**A native programming language for scientific computing.** Oct brings dimensional analysis, matrix math, embedded tests, and native compilation into a single, legible language. Written in Go, Oct compiles down to standalone native binaries so researchers never have to maintain separate prototype and production codebases.

Oct eliminates the two-language problem: write your model once, test it in place, and distribute a single executable.

> **Latest Release:** Oct 1.1.0 — Read the [Release Notes](docs/releases/OCT_1_1_RELEASE_NOTES.md) or see the [Installation Guide](docs/releases/INSTALL_1_1.md).

## Why Oct?

Scientific software usually forces a trade-off: fast prototyping in Python at the cost of performance, or speed and distribution in C++/Rust at the cost of high language friction. Go offers an excellent runtime and distribution model, but lacks explicit domain abstractions for scientific contracts.

Oct closes the gap. It provides domain-first language primitives while using Go as a runtime foundation and native backend:

* **First-Class Scientific Types:** Built-in SI dimensions, vectors/matrices/tensors, and explicit error handling.
* **Embedded Verification:** Native xUnit.NET style `[Fact]` unit tests, benchmarks, typed artifacts, and reproducible lockfiles ship alongside your code.
* **Native Executables:** Compiles directly to standalone binaries via the Go backend, with optional Octxiliary sidecars for standard Go library interop.
* **Readable & Deterministic:** Clear control flow, explicit generic instantiation, and self-documenting code built for human review and reliable LLM generation.

## A small Oct program

```oct
package ReadmeDemo

concept Distance = Float<m>
concept Duration = Float<s>
concept Speed = Float<m/s>

template record Measurement<T> {
    Value: T
    UnitLabel: String
}

fn AverageSpeed(distance: Distance, elapsed: Duration) -> Measurement<Speed> ! Error {
    if elapsed <= 0.0s {
        return error("elapsed time must be positive")
    }
    return Measurement<Speed> {
        Value: distance / elapsed
        UnitLabel: "m/s"
    }
}

[Fact]
fn MeasuresSpeed() -> Void ! Error {
    let result = AverageSpeed(12.0m, 3.0s)?
    Assert.Equal(4.0m/s, result.Value, "distance divided by time")
}
```

`concept` gives domain quantities names without hiding their units. `template` makes the measurement shape reusable while each application has a concrete type. The division has to produce speed, and the test exercises the same code that can compile to a native executable. Save this as `readme.octest` and run `oct test readme.octest --execution compiled`.

## More than the scientific core

Oct also explores what a well-equipped programming language can do around the computation itself. It has FLOW-backed queries over arrays and tables, package and artifact workflows, document generation, Go integration and source-generation experiments, and build orchestration that can coordinate Go, C/C++, and Rust projects. These capabilities serve the scientific program; they are not prerequisites for writing one.

The [language reference](Language/reference/README.md) and [executable contracts](Language/README.md) define the supported Oct surface. Some wider work, including `oct make`, OctGen, SDSL-V, Prometheus, and alternate backends, is experimental or separately governed. The [1.0 surface manifest](docs/releases/OCT_1_0_SURFACE_MANIFEST.md) identifies the stable boundary; this release does not silently make every experiment a stable API.

## Get started

Install the 1.1.0 CLI with Go:

```sh
go install github.com/yuechen-li-dev/oct/cmd/oct@v1.1.0
oct version
```

From a checkout of this repository:

```sh
go run ./cmd/oct --help
go run ./cmd/oct test Examples/SmartGreenhouseController --execution compiled
```

To start a project with an installed CLI:

```sh
oct new library HelloScience
cd HelloScience
oct test .
```

Native compilation uses the Go toolchain. Release archives and their installation details are in the [1.1 installation guide](docs/releases/INSTALL_1_1.md).

## Explore the repository

- [Examples](Examples/README.md) — small programs and complete workflows.
- [Science libraries](docs/science/README.md) — numerical, physical, statistical, and domain packages by task.
- [Documentation](docs/README.md) — CLI, architecture, testing, and development guides.
- [Language reference](Language/reference/README.md) — authoritative syntax and supported features.
- [Language contracts](Language/README.md) — valid and invalid executable specimens.

Concept Vulkan language and compiler development has moved to the [Concept repository](https://github.com/yuechen-li-dev/Concept). Oct retains the historical Prometheus artifacts that consume its output.
