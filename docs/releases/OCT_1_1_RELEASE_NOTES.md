# Oct 1.1.0

Oct 1.1 extends the scientific language and Go-backed toolchain released in 1.0.0. The release adds ways to name and reuse domain contracts, query and store typed data, build new artifacts, and target more execution environments. The scientific core remains the reason for the language; the wider tools make it easier to carry a model into a complete program.

## Language and data

- **Concepts and explicit templates** give scientific values and reusable records, functions, flows, and queries typed names. Refined concepts can enforce bounded requirements at admission; template applications specialize to concrete declarations before ordinary checking and execution.
- **More capable FLOW** adds explicit captures, turn inputs and yielded values, shared compiled expression lowering, and `async fn` / `await` authoring that lowers to Octomata state machines.
- **FLOW-backed queries** operate over arrays and record tables, with typed table loading, immutable updates, static column projections, and proof-aware binary-search indexes. Octagon loading gained typed payload enum literals and compiled numeric and columnar parity.
- **Oct-XML** provides typed template markup for document authoring.

## Execution and compiler

- The compiled Go path gained source-only emission, better generated support code, broader FLOW parity, deterministic range workers for batch execution, and opt-in constant optimization.
- The direct MIR-to-WebAssembly path gained a build target and deterministic execution coverage. The experimental Verilog profile grew from combinational output to sequential FLOW state machines.
- Compiler work also improved diagnostics, template specialization traces, and inspectable CFG, reaching-definitions, and MIR examples.

## Scientific libraries and artifacts

- Numerical, geometric, algorithmic, and domain libraries expanded, with clearer ownership across mathematics, numerics, optimization, simulation, physics, RF, and units. Domain contracts increasingly use typed units and concepts.
- **OctCument** grew from immutable document values to semantic figures and references, templates and styles, anchored layout, Markdown and DOCX output, and LaTeX/PDF artifacts. **Atlas** adds opt-in semantic documentation graphs linking claims, implementation, evidence, and artifacts.
- Artifact output now records stronger provenance and explicit native capability requests and host grants.

## Tooling and research surfaces

- OctGo companion declarations and `oct check` strengthen typed Go integration. Experimental OctGen can generate Go source from checked Oct models; template discovery exposes a catalog of explicit specializations.
- Build and project tooling expanded around native integration, including Go and C/C++ build workflows. SDSL-V conformance and Prometheus GPU/model research advanced substantially; these remain separately governed research surfaces, outside the stable Oct 1.x language promise.
- CI, repository layout, documentation, and examples were stabilized and organized. Concept Vulkan language and compiler development graduated to the [Concept repository](https://github.com/yuechen-li-dev/Concept); Oct keeps only historical consumer artifacts.

The [1.0 surface manifest](OCT_1_0_SURFACE_MANIFEST.md) remains the explicit guide to stable versus experimental APIs. See the [full comparison](https://github.com/yuechen-li-dev/oct/compare/v1.0.0...v1.1.0) for the complete change history and [installation guide](INSTALL_1_1.md) for release archives and prerequisites.
