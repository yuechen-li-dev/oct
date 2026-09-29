# Oct

**A programming language for science.** Oct brings physical units, numerical data, tests, artifacts, and native programs into one readable language. Its compiler and native backend are implemented in Go, so an Oct program can become an ordinary executable without asking a scientist to maintain a second implementation of the same model.

Oct is deliberately ambitious: a scientific idea should be able to grow from a calculation into a tested, reviewable, distributable system without changing its vocabulary halfway through.

**Oct 1.1.0.** See the [release notes](docs/releases/OCT_1_1_RELEASE_NOTES.md) for what has changed since 1.0.0 and the [installation guide](docs/releases/INSTALL_1_1.md) for the supported binary archives.

## Why Oct exists

Scientific software often has a two-language problem: a productive language for exploring a model, then a lower-level language for the parts that must run quickly or ship as a native program. Python makes exploration approachable but often pushes performance-critical work into extensions. C++ and Rust offer native control but ask researchers to carry more systems complexity through everyday scientific code. Go provides a practical implementation and distribution foundation, but its language does not express all the scientific contracts we want at the source level.

Oct is an attempt to close that gap. The scientist writes Oct for both the model and the program around it. Go implements the language and its current native compilation path; Oct adds the domain vocabulary and checks that Go alone does not provide. The aim is to make the code a human can review also be the code an LLM can author, test, and hand back as a reproducible artifact.

That goal shapes the language:

- **Scientific meaning is explicit.** SI dimensions, arrays, vectors, matrices, tensors, and fallible results are part of the type and execution model.
- **Evidence travels with the program.** `[Fact]` tests, benchmarks, typed artifacts, packages, and optional lockfiles make results repeatable and inspectable.
- **Native delivery is ordinary.** The Go backend builds executables, while explicit Octxiliary sidecars provide access to selected Go libraries when a wrapper is needed.
- **The source stays legible.** Named concepts, explicit template applications, and visible control flow let reviewers see what a value means and where a decision came from.

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
