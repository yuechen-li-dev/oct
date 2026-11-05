# Language coverage map

[Reference home](README.md) · [Maintenance](maintaining.md)

This is an ownership inventory for the implemented language, not a list of
future proposals. It is audited against source declaration/parser forms,
every current Oct contract area, and the library directory. Contracts specify
their own scope and execution requirements; a row does not claim all possible
programs or all targets are qualified.

## Declarations and authoring surfaces

| Surface | Reference owner |
| --- | --- |
| `package`, `import`, source selection, and manifests | [Packages](language/packages-and-imports.md), [package tooling](tooling/packages-and-dependencies.md) |
| `fn`, entry functions, explicit function types/captures, and recursion | [Functions](language/functions.md), [function values](language/function-values.md) |
| Product declarations, tables, nominal identity, and `with` | [Records](language/records-and-tables.md) |
| `enum`, payload construction, exhaustive `match`, and `Option<T>` | [Enums](language/enums-and-options.md) |
| `concept`, `Self`, `Require`, checked refinements, capability requests | [Concepts](language/concepts-and-refinements.md) |
| `template` records/functions/flows/queries, selectors, and catalog | [Templates](language/templates-and-selectors.md) |
| `flow`, `state`, `board`, guards, transfers, resume slots, and observations | [Octomata](runtime/flows-and-state-machines.md) |
| `accepts`, `yields`, turn scope, recursive persistent values, and host checkpoints | [Turns](runtime/turns-and-checkpoints.md) |
| `async fn` and `await` | [Async](runtime/async.md) |
| `query`, `filter`, `map`, `take`, and consuming terminals | [Queries](runtime/queries.md) |
| Ranked decisions, enum judgments, and persistent policy settings | [Utility](language/utility-selection.md), [controller policy](runtime/utility-policy.md) |
| Scientific literals, dimensions, collections, contractions, tensor/differential expressions | [Units](language/units.md), [arrays](language/arrays.md), [mathematical values](language/vectors-matrices-and-tensors.md) |
| Testing/lane attributes, expected failures, artifact capability provider, static assertions | [Testing](tooling/testing.md), [publication](tooling/artifacts-and-benchmarks.md) |
| `go fn`, source emission, native manifests, and special Make attributes | [Interop](tooling/interoperability.md) |
| `profile Verilog`, native/WASM targets, and explicit subset refusals | [Profiles](language/compilation-profiles.md), [targets](tooling/targets.md) |
| Typed markup, documents, and semantic documentation graphs | [Oct-XML](language/typed-markup.md), [Document](language/standard-libraries.md#document-octcument-m1), [Atlas](tooling/atlas.md) |
| Data-only Octagon and typed JSON | [Octagon](tooling/octagon.md), [JSON](language/standard-libraries.md#json) |
| SDSL-V and GPU `tensor` statements, Prometheus, UI host protocol | [SDSL-V](../../docs/SDSL_V_LANGUAGE_SPEC.md), [Prometheus](runtime/prometheus.md), [UIBridge](tooling/uibridge-protocol.md) |

## Ordinary AST statement and expression forms

The comments below name compiler node kinds for maintenance checks. They are
an inventory, not source syntax. Internal/rejected forms are identified rather
than presented as supported language features.

| Forms | Reference / boundary |
| --- | --- |
| Bindings and assignment <!-- ast-coverage: LetStmt VarStmt AssignStmt IndexAssignStmt --> | [Variables](language/variables-and-values.md), [arrays](language/arrays.md) |
| Board writes <!-- ast-coverage: FieldAssignStmt FieldIndexAssignStmt --> | [Boards](runtime/flows-and-state-machines.md) |
| Returns and expression statements <!-- ast-coverage: ReturnStmt ExprStmt --> | [Functions](language/functions.md) |
| Tuple/destructuring assignment <!-- ast-coverage: DestructureAssignStmt --> | Rejected source form; return a [record](language/records-and-tables.md) |
| Loops and branches <!-- ast-coverage: ForStmt WhileStmt IfStmt MatchStmt --> | [Control flow](language/control-flow.md), [fallible statement match](language/errors.md) |
| State transfers and guards <!-- ast-coverage: GotoStmt SuspendStmt YieldStmt RememberStmt ResumeStmt WhenStmt --> | [Octomata](runtime/flows-and-state-machines.md), [turns](runtime/turns-and-checkpoints.md) |
| Accelerator block <!-- ast-coverage: PrometheusStmt --> | [Experimental Prometheus](runtime/prometheus.md) |
| Primitive values <!-- ast-coverage: IntegerLiteral FloatLiteral BoolLiteral StringLiteralExpr IdentifierExpr --> | [Lexical structure](language/lexical-structure.md), [types](language/types.md) |
| Calls and explicit functions <!-- ast-coverage: CallExpr FunctionExpr AwaitExpr --> | [Functions](language/functions.md), [captures](language/function-values.md), [async](runtime/async.md) |
| Collections, indexing, and fields <!-- ast-coverage: ArrayLiteralExpr RepeatExpr VectorLiteralExpr MatrixLiteralExpr IndexExpr FieldAccessExpr --> | [Arrays](language/arrays.md), [mathematical values](language/vectors-matrices-and-tensors.md), [records](language/records-and-tables.md) |
| Operators and ranges <!-- ast-coverage: BinaryExpr UnaryExpr RangeExpr ParenExpr --> | [Expressions](language/expressions.md) |
| Failure handling <!-- ast-coverage: PropagateExpr UnwrapExpr --> | [Errors](language/errors.md) |
| Value selection <!-- ast-coverage: SwitchExpr MatchExpr IfExpr UtilityWhenExpr --> | [Control flow](language/control-flow.md), [enums](language/enums-and-options.md), [utility](language/utility-selection.md) |
| Parallel mapping <!-- ast-coverage: BatchExpr --> | [Batch](runtime/batch.md) |
| Product construction and update <!-- ast-coverage: RecordLiteralExpr RecordUpdateExpr --> | [Records](language/records-and-tables.md) |
| Typed field references <!-- ast-coverage: SelectorExpr --> | [Selectors](language/templates-and-selectors.md#typed-selectors) |
| Enum / option construction <!-- ast-coverage: EnumValueExpr --> | [Enums](language/enums-and-options.md) |
| Typed markup <!-- ast-coverage: MarkupElementExpr --> | [Oct-XML](language/typed-markup.md) |

Query declarations elaborate to FLOW; async and templates elaborate before
ordinary execution. Octagon has a deliberately restricted data parser.
Compiler AST storage for a form does not itself make that form legal source.

## Executable contract areas

Every current Oct contract area has an owner below. The documentation checker
fails if a new area appears without a row, or if a row outlives its area.
Run a concrete test package/fixture, not an arbitrary parent grouping directory.
See [the full corpus runner](../README.md#running-the-corpus) for all directories
and their special execution prerequisites.

| Contract area | Reference owner | Executable specimens |
| --- | --- | --- |
| Builtins / Entropy | [Standard Libraries](language/standard-libraries.md) | [Language/Builtins/Entropy](../Builtins/Entropy/) |
| Builtins / FloatClamp01 | [Builtins](language/builtins.md) | [Language/Builtins/FloatClamp01](../Builtins/FloatClamp01/) |
| Builtins / Json | [Standard Libraries](language/standard-libraries.md) | [Language/Builtins/Json](../Builtins/Json/) |
| Concurrency / Batch | [Batch](runtime/batch.md) | [Language/Concurrency/Batch](../Concurrency/Batch/) |
| ControlFlow / AsyncM0 | [Async functions](runtime/async.md) | [Language/ControlFlow/AsyncM0](../ControlFlow/AsyncM0/) |
| ControlFlow / CompiledSwitchExpr | [Control Flow](language/control-flow.md) | [Language/ControlFlow/CompiledSwitchExpr](../ControlFlow/CompiledSwitchExpr/) |
| ControlFlow / ConditionSwitch | [Control Flow](language/control-flow.md) | [Language/ControlFlow/ConditionSwitch](../ControlFlow/ConditionSwitch/) |
| ControlFlow / DecisionLadder | [Control Flow](language/control-flow.md) | [Language/ControlFlow/DecisionLadder](../ControlFlow/DecisionLadder/) |
| ControlFlow / EnumPayloadMatchCompiled | [Enums](language/enums-and-options.md) | [Language/ControlFlow/EnumPayloadMatchCompiled](../ControlFlow/EnumPayloadMatchCompiled/) |
| ControlFlow / IfExpression | [Control Flow](language/control-flow.md) | [Language/ControlFlow/IfExpression](../ControlFlow/IfExpression/) |
| ControlFlow / Loops | [Control Flow](language/control-flow.md) | [Language/ControlFlow/Loops](../ControlFlow/Loops/) |
| ControlFlow / OctomataBlackboardM6 | [Octomata](runtime/flows-and-state-machines.md) | [Language/ControlFlow/OctomataBlackboardM6](../ControlFlow/OctomataBlackboardM6/) |
| ControlFlow / OctomataBoardAggregatesM0 | [Octomata](runtime/flows-and-state-machines.md) | [Language/ControlFlow/OctomataBoardAggregatesM0](../ControlFlow/OctomataBoardAggregatesM0/) |
| ControlFlow / OctomataBoardIndexedAssignment | [Octomata](runtime/flows-and-state-machines.md) | [Language/ControlFlow/OctomataBoardIndexedAssignment](../ControlFlow/OctomataBoardIndexedAssignment/) |
| ControlFlow / OctomataBoardSnapshot | [Octomata](runtime/flows-and-state-machines.md) | [Language/ControlFlow/OctomataBoardSnapshot](../ControlFlow/OctomataBoardSnapshot/) |
| ControlFlow / OctomataCheckpointDeterminism | [Turns, yields, and checkpoints](runtime/turns-and-checkpoints.md) | [Language/ControlFlow/OctomataCheckpointDeterminism](../ControlFlow/OctomataCheckpointDeterminism/) |
| ControlFlow / OctomataCompiledBoundary | [Octomata](runtime/flows-and-state-machines.md) | [Language/ControlFlow/OctomataCompiledBoundary](../ControlFlow/OctomataCompiledBoundary/) |
| ControlFlow / OctomataCoreA | [Octomata](runtime/flows-and-state-machines.md) | [Language/ControlFlow/OctomataCoreA](../ControlFlow/OctomataCoreA/) |
| ControlFlow / OctomataCoreB | [Octomata](runtime/flows-and-state-machines.md) | [Language/ControlFlow/OctomataCoreB](../ControlFlow/OctomataCoreB/) |
| ControlFlow / OctomataFallibilityM0 | [Octomata](runtime/flows-and-state-machines.md) | [Language/ControlFlow/OctomataFallibilityM0](../ControlFlow/OctomataFallibilityM0/) |
| ControlFlow / OctomataFlowCallExpr | [Octomata](runtime/flows-and-state-machines.md) | [Language/ControlFlow/OctomataFlowCallExpr](../ControlFlow/OctomataFlowCallExpr/) |
| ControlFlow / OctomataFlowEnumConstructors | [Octomata](runtime/flows-and-state-machines.md) | [Language/ControlFlow/OctomataFlowEnumConstructors](../ControlFlow/OctomataFlowEnumConstructors/) |
| ControlFlow / OctomataFlowExprParity | [Octomata](runtime/flows-and-state-machines.md) | [Language/ControlFlow/OctomataFlowExprParity](../ControlFlow/OctomataFlowExprParity/) |
| ControlFlow / OctomataFlowIfExpr | [Octomata](runtime/flows-and-state-machines.md) | [Language/ControlFlow/OctomataFlowIfExpr](../ControlFlow/OctomataFlowIfExpr/) |
| ControlFlow / OctomataFlowIndexExpr | [Octomata](runtime/flows-and-state-machines.md) | [Language/ControlFlow/OctomataFlowIndexExpr](../ControlFlow/OctomataFlowIndexExpr/) |
| ControlFlow / OctomataFlowLocals | [Octomata](runtime/flows-and-state-machines.md) | [Language/ControlFlow/OctomataFlowLocals](../ControlFlow/OctomataFlowLocals/) |
| ControlFlow / OctomataFlowMatchExpr | [Octomata](runtime/flows-and-state-machines.md) | [Language/ControlFlow/OctomataFlowMatchExpr](../ControlFlow/OctomataFlowMatchExpr/) |
| ControlFlow / OctomataFlowNestedTransferM1 | [Octomata](runtime/flows-and-state-machines.md) | [Language/ControlFlow/OctomataFlowNestedTransferM1](../ControlFlow/OctomataFlowNestedTransferM1/) |
| ControlFlow / OctomataFlowRecordLiteral | [Octomata](runtime/flows-and-state-machines.md) | [Language/ControlFlow/OctomataFlowRecordLiteral](../ControlFlow/OctomataFlowRecordLiteral/) |
| ControlFlow / OctomataFlowTurnM0 | [Turns, yields, and checkpoints](runtime/turns-and-checkpoints.md) | [Language/ControlFlow/OctomataFlowTurnM0](../ControlFlow/OctomataFlowTurnM0/) |
| ControlFlow / OctomataFlowTurnM1 | [Turns, yields, and checkpoints](runtime/turns-and-checkpoints.md) | [Language/ControlFlow/OctomataFlowTurnM1](../ControlFlow/OctomataFlowTurnM1/) |
| ControlFlow / OctomataFlowUnsupportedCall | [Octomata](runtime/flows-and-state-machines.md) | [Language/ControlFlow/OctomataFlowUnsupportedCall](../ControlFlow/OctomataFlowUnsupportedCall/) |
| ControlFlow / OctomataObservability | [Octomata](runtime/flows-and-state-machines.md) | [Language/ControlFlow/OctomataObservability](../ControlFlow/OctomataObservability/) |
| ControlFlow / OctomataResumeM57 | [Octomata](runtime/flows-and-state-machines.md) | [Language/ControlFlow/OctomataResumeM57](../ControlFlow/OctomataResumeM57/) |
| ControlFlow / OctomataUtilityWhen | [Controller utility policy](runtime/utility-policy.md) | [Language/ControlFlow/OctomataUtilityWhen](../ControlFlow/OctomataUtilityWhen/) |
| Data / Octagon | [octagon](tooling/octagon.md) | [Language/Data/Octagon](../Data/Octagon/) |
| Data / QueryM0 | [FLOW-backed queries](runtime/queries.md) | [Language/Data/QueryM0](../Data/QueryM0/) |
| Errors / Fallible | [Errors](language/errors.md) | [Language/Errors/Fallible](../Errors/Fallible/) |
| Expressions / Arithmetic | [Expressions](language/expressions.md) | [Language/Expressions/Arithmetic](../Expressions/Arithmetic/) |
| Expressions / ArrayScalarBroadcast | [Expressions](language/expressions.md) | [Language/Expressions/ArrayScalarBroadcast](../Expressions/ArrayScalarBroadcast/) |
| Expressions / ArrayWhere | [Expressions](language/expressions.md) | [Language/Expressions/ArrayWhere](../Expressions/ArrayWhere/) |
| Expressions / ArrowUnificationMx102 | [Expressions](language/expressions.md) | [Language/Expressions/ArrowUnificationMx102](../Expressions/ArrowUnificationMx102/) |
| Expressions / CallableConsumersM0 | [Function Values](language/function-values.md) | [Language/Expressions/CallableConsumersM0](../Expressions/CallableConsumersM0/) |
| Expressions / DifferentialOperatorsM0 | [Vectors, Matrices, and Tensors](language/vectors-matrices-and-tensors.md) | [Language/Expressions/DifferentialOperatorsM0](../Expressions/DifferentialOperatorsM0/) |
| Expressions / DifferentialOperatorsM1 | [Vectors, Matrices, and Tensors](language/vectors-matrices-and-tensors.md) | [Language/Expressions/DifferentialOperatorsM1](../Expressions/DifferentialOperatorsM1/) |
| Expressions / Literals | [Expressions](language/expressions.md) | [Language/Expressions/Literals](../Expressions/Literals/) |
| Expressions / Logical | [Expressions](language/expressions.md) | [Language/Expressions/Logical](../Expressions/Logical/) |
| Expressions / MatrixConstructionMx100a | [Vectors, Matrices, and Tensors](language/vectors-matrices-and-tensors.md) | [Language/Expressions/MatrixConstructionMx100a](../Expressions/MatrixConstructionMx100a/) |
| Expressions / OctXMLM0 | [Oct-XML typed template markup](language/typed-markup.md) | [Language/Expressions/OctXMLM0](../Expressions/OctXMLM0/) |
| Expressions / RecordFieldAccessChains | [Records](language/records-and-tables.md) | [Language/Expressions/RecordFieldAccessChains](../Expressions/RecordFieldAccessChains/) |
| Expressions / RecordUpdateM9 | [Records](language/records-and-tables.md) | [Language/Expressions/RecordUpdateM9](../Expressions/RecordUpdateM9/) |
| Expressions / RepeatedElements | [Arrays](language/arrays.md) | [Language/Expressions/RepeatedElements](../Expressions/RepeatedElements/) |
| Expressions / ScientificCalculatorM72 | [Expressions](language/expressions.md) | [Language/Expressions/ScientificCalculatorM72](../Expressions/ScientificCalculatorM72/) |
| Expressions / ScientificCalculatorM73 | [Expressions](language/expressions.md) | [Language/Expressions/ScientificCalculatorM73](../Expressions/ScientificCalculatorM73/) |
| Expressions / TensorEinsteinM0 | [Vectors, Matrices, and Tensors](language/vectors-matrices-and-tensors.md) | [Language/Expressions/TensorEinsteinM0](../Expressions/TensorEinsteinM0/) |
| Expressions / TensorEinsteinM1 | [Vectors, Matrices, and Tensors](language/vectors-matrices-and-tensors.md) | [Language/Expressions/TensorEinsteinM1](../Expressions/TensorEinsteinM1/) |
| Expressions / TensorEinsteinM3 | [Vectors, Matrices, and Tensors](language/vectors-matrices-and-tensors.md) | [Language/Expressions/TensorEinsteinM3](../Expressions/TensorEinsteinM3/) |
| Expressions / TensorEinsteinM4 | [Vectors, Matrices, and Tensors](language/vectors-matrices-and-tensors.md) | [Language/Expressions/TensorEinsteinM4](../Expressions/TensorEinsteinM4/) |
| Expressions / TensorEinsteinM5 | [Vectors, Matrices, and Tensors](language/vectors-matrices-and-tensors.md) | [Language/Expressions/TensorEinsteinM5](../Expressions/TensorEinsteinM5/) |
| Expressions / TensorEinsteinM6 | [Vectors, Matrices, and Tensors](language/vectors-matrices-and-tensors.md) | [Language/Expressions/TensorEinsteinM6](../Expressions/TensorEinsteinM6/) |
| Expressions / TensorEinsteinM7 | [Vectors, Matrices, and Tensors](language/vectors-matrices-and-tensors.md) | [Language/Expressions/TensorEinsteinM7](../Expressions/TensorEinsteinM7/) |
| Expressions / TensorUtilitiesM4 | [Vectors, Matrices, and Tensors](language/vectors-matrices-and-tensors.md) | [Language/Expressions/TensorUtilitiesM4](../Expressions/TensorUtilitiesM4/) |
| Expressions / UtilityWhen | [Utility selection](language/utility-selection.md) | [Language/Expressions/UtilityWhen](../Expressions/UtilityWhen/) |
| Functions / Calls | [Functions](language/functions.md) | [Language/Functions/Calls](../Functions/Calls/) |
| Functions / CommentsAndDocs | [Lexical](language/lexical-structure.md) | [Language/Functions/CommentsAndDocs](../Functions/CommentsAndDocs/) |
| Functions / ContextualKeywords | [Lexical](language/lexical-structure.md) | [Language/Functions/ContextualKeywords](../Functions/ContextualKeywords/) |
| Functions / FunctionValues | [Function Values](language/function-values.md) | [Language/Functions/FunctionValues](../Functions/FunctionValues/) |
| Mechanics / ContinuumM0 | [Vectors, Matrices, and Tensors](language/vectors-matrices-and-tensors.md) | [Language/Mechanics/ContinuumM0](../Mechanics/ContinuumM0/) |
| Mechanics / ContinuumM1 | [Vectors, Matrices, and Tensors](language/vectors-matrices-and-tensors.md) | [Language/Mechanics/ContinuumM1](../Mechanics/ContinuumM1/) |
| Mechanics / ContinuumM2 | [Vectors, Matrices, and Tensors](language/vectors-matrices-and-tensors.md) | [Language/Mechanics/ContinuumM2](../Mechanics/ContinuumM2/) |
| Mechanics / ContinuumM3 | [Vectors, Matrices, and Tensors](language/vectors-matrices-and-tensors.md) | [Language/Mechanics/ContinuumM3](../Mechanics/ContinuumM3/) |
| Mechanics / ContinuumM4 | [Vectors, Matrices, and Tensors](language/vectors-matrices-and-tensors.md) | [Language/Mechanics/ContinuumM4](../Mechanics/ContinuumM4/) |
| Mechanics / ContinuumM5 | [Vectors, Matrices, and Tensors](language/vectors-matrices-and-tensors.md) | [Language/Mechanics/ContinuumM5](../Mechanics/ContinuumM5/) |
| Mechanics / ContinuumM6 | [Vectors, Matrices, and Tensors](language/vectors-matrices-and-tensors.md) | [Language/Mechanics/ContinuumM6](../Mechanics/ContinuumM6/) |
| Packages / CrossPackageM81 | [Packages](language/packages-and-imports.md) | [Language/Packages/CrossPackageM81](../Packages/CrossPackageM81/) |
| Packages / ImportedRecordIdentity | [Packages](language/packages-and-imports.md) | [Language/Packages/ImportedRecordIdentity](../Packages/ImportedRecordIdentity/) |
| Profiles / VerilogM0 | [Compilation profiles](language/compilation-profiles.md) | [Language/Profiles/VerilogM0](../Profiles/VerilogM0/) |
| Profiles / VerilogM1 | [Compilation profiles](language/compilation-profiles.md) | [Language/Profiles/VerilogM1](../Profiles/VerilogM1/) |
| Profiles / VerilogM2 | [Compilation profiles](language/compilation-profiles.md) | [Language/Profiles/VerilogM2](../Profiles/VerilogM2/) |
| ReferenceExamples | [Learn and use Oct](README.md) | [Language/ReferenceExamples](../ReferenceExamples/) |
| Runtime / LibraryBuiltins | [Standard Libraries](language/standard-libraries.md) | [Language/Runtime/LibraryBuiltins](../Runtime/LibraryBuiltins/) |
| Testing / CompiledArrayLowering | [octest](tooling/testing.md) | [Language/Testing/CompiledArrayLowering](../Testing/CompiledArrayLowering/) |
| Testing / CompiledAssertions | [octest](tooling/testing.md) | [Language/Testing/CompiledAssertions](../Testing/CompiledAssertions/) |
| Testing / CompiledBatchCapture | [octest](tooling/testing.md) | [Language/Testing/CompiledBatchCapture](../Testing/CompiledBatchCapture/) |
| Testing / CompiledBuiltinSweep | [octest](tooling/testing.md) | [Language/Testing/CompiledBuiltinSweep](../Testing/CompiledBuiltinSweep/) |
| Testing / CompiledCallbacks | [octest](tooling/testing.md) | [Language/Testing/CompiledCallbacks](../Testing/CompiledCallbacks/) |
| Testing / CompiledIdentifierHygiene | [octest](tooling/testing.md) | [Language/Testing/CompiledIdentifierHygiene](../Testing/CompiledIdentifierHygiene/) |
| Testing / CompiledOctxiliary | [octest](tooling/testing.md) | [Language/Testing/CompiledOctxiliary](../Testing/CompiledOctxiliary/) |
| Testing / CompiledPi | [octest](tooling/testing.md) | [Language/Testing/CompiledPi](../Testing/CompiledPi/) |
| Testing / CompiledSelectedReachable | [octest](tooling/testing.md) | [Language/Testing/CompiledSelectedReachable](../Testing/CompiledSelectedReachable/) |
| Testing / CompiledStringBuiltins | [octest](tooling/testing.md) | [Language/Testing/CompiledStringBuiltins](../Testing/CompiledStringBuiltins/) |
| Testing / InterpretedOctxiliary | [octest](tooling/testing.md) | [Language/Testing/InterpretedOctxiliary](../Testing/InterpretedOctxiliary/) |
| Testing / LaneAttributes | [octest](tooling/testing.md) | [Language/Testing/LaneAttributes](../Testing/LaneAttributes/) |
| Testing / OctestAssertionsM83 | [octest](tooling/testing.md) | [Language/Testing/OctestAssertionsM83](../Testing/OctestAssertionsM83/) |
| Testing / SelectedDirectoryCompiled | [octest](tooling/testing.md) | [Language/Testing/SelectedDirectoryCompiled](../Testing/SelectedDirectoryCompiled/) |
| Testing / SelectedFileCompiled | [octest](tooling/testing.md) | [Language/Testing/SelectedFileCompiled](../Testing/SelectedFileCompiled/) |
| Testing / TheoryCycleTime | [octest](tooling/testing.md) | [Language/Testing/TheoryCycleTime](../Testing/TheoryCycleTime/) |
| Tooling / Artifacts | [Artifacts, static checks, and benchmarks](tooling/artifacts-and-benchmarks.md) | [Language/Tooling/Artifacts](../Tooling/Artifacts/) |
| Tooling / CompiledData | [Artifacts, static checks, and benchmarks](tooling/artifacts-and-benchmarks.md) | [Language/Tooling/CompiledData](../Tooling/CompiledData/) |
| Tooling / ConceptCapabilitiesM2 | [Concepts](language/concepts-and-refinements.md) | [Language/Tooling/ConceptCapabilitiesM2](../Tooling/ConceptCapabilitiesM2/) |
| Tooling / JsonInfer | [CLI](tooling/cli.md) | [Language/Tooling/JsonInfer](../Tooling/JsonInfer/) |
| Types / Arrays | [Arrays](language/arrays.md) | [Language/Types/Arrays](../Types/Arrays/) |
| Types / Bytes | [Types](language/types.md) | [Language/Types/Bytes](../Types/Bytes/) |
| Types / ComplexM0 | [Types](language/types.md) | [Language/Types/ComplexM0](../Types/ComplexM0/) |
| Types / ComplexM0a | [Types](language/types.md) | [Language/Types/ComplexM0a](../Types/ComplexM0a/) |
| Types / ConceptsM0 | [Concepts](language/concepts-and-refinements.md) | [Language/Types/ConceptsM0](../Types/ConceptsM0/) |
| Types / ConceptsM1 | [Concepts](language/concepts-and-refinements.md) | [Language/Types/ConceptsM1](../Types/ConceptsM1/) |
| Types / DeclaredNumericTypes | [Types](language/types.md) | [Language/Types/DeclaredNumericTypes](../Types/DeclaredNumericTypes/) |
| Types / EnumsAssociated | [Enums](language/enums-and-options.md) | [Language/Types/EnumsAssociated](../Types/EnumsAssociated/) |
| Types / Option | [Enums](language/enums-and-options.md) | [Language/Types/Option](../Types/Option/) |
| Types / ParametricsM0 | [Parametric templates](language/templates-and-selectors.md) | [Language/Types/ParametricsM0](../Types/ParametricsM0/) |
| Types / Ranges | [Expressions](language/expressions.md) | [Language/Types/Ranges](../Types/Ranges/) |
| Types / RecordTables | [Records](language/records-and-tables.md) | [Language/Types/RecordTables](../Types/RecordTables/) |
| Types / TemplateTortureM0 | [Parametric templates](language/templates-and-selectors.md) | [Language/Types/TemplateTortureM0](../Types/TemplateTortureM0/) |
| Types / Tuples | [Types](language/types.md) | [Language/Types/Tuples](../Types/Tuples/) |
| Types / UnitsM1 | [Units](language/units.md) | [Language/Types/UnitsM1](../Types/UnitsM1/) |
| Types / VectorsMatricesM92 | [Vectors, Matrices, and Tensors](language/vectors-matrices-and-tensors.md) | [Language/Types/VectorsMatricesM92](../Types/VectorsMatricesM92/) |

## Library and specialized inventories

Every current library directory is listed in [Libraries](libraries.md).
Library source/tests remain the exact API contracts for individual routines;
the reference supplies language rules, feature ownership, and preferred routes.
[SDSL-V](../SDSL-V/README.md) has its own language/conformance inventory and
does not inherit ordinary Oct legality from this map.

## Audit corrections and explicit limits

The November 2026 organization pass corrected these disagreements rather than
silently preserving them:

- Global rules denied all numeric admission while `Types` already specified
  declared Float/Complex widening. `Types` owns the exact contextual rules.
- Expressions described `Array.Where` as future work although it is implemented.
- Record-table prose excluded queries although the query contracts accept tables.
- Builtin prose denied user templates even though explicit typed templates exist.
- The vector chapter included SDSL-V statement syntax and a stale lowering milestone.
- FLOW guard rules omitted the bounded action blocks documented later in the chapter.
- Introductory function/FLOW chapters opened with async machinery rather than basics.
- CLI parent-corpus examples could suggest zero-positive-case runs were full conformance.
- Package scaffolding omitted application kind, explicit output paths, and
  kind-based default directories; manifest reference omitted author/date metadata.
- The production `FFT` builtin lacked reference coverage; it now has a runnable
  example checking the transform and its rejected input boundaries.

Existing backend gaps stay explicit in the owning chapters: unsupported
sidecar builtins, bounded async/Require/template forms, non-persistent function
values, backend-specific type limits, and the source-only float-helper omission.
The audit does not claim every historical fragment was executed. Full lesson
blocks carry checked source provenance; detailed historical examples link to
their owning contracts and retain their stated boundaries.
