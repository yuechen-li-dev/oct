# Oct utility selection and validated record tables

`when utility` was already first-class Oct before this detour. The table
vertical adds validated columnar data; the continuation adds ordinary
scientific APIs that derive small, inspectable utility scores from evidence.
Selection and fitting remain separate. There is no second policy syntax, LLM
call, online learner, or hidden runtime model.

## `when utility`

```oct
let selected = when utility {
    case cooperative when cooperativeEligible score cooperativeScore
    case selective when selectiveEligible score selectiveScore
    else fullFp32
}
```

Cases are visited in source order. A condition is evaluated once; false skips
both score and value. Every eligible case evaluates one dimensionless `Int`
score and one candidate value. The highest score wins and ties keep the first
case. The required `else` is evaluated only when no case is eligible, so the
no-eligible result is explicit. Candidate and fallback values must share one
type and that type is the expression result.

This deliberately retains Oct's existing score envelope. Scores are `Int`, so
NaN is unrepresentable and there is no host floating-point comparison behavior
to pin. Ordinary authored or learned weights remain ordinary records; callers
can convert a bounded numeric score to `Int` explicitly with Oct's named
rounding operations. The core expression returns the chosen `T` directly and
does not allocate an evidence list or synthesize a generic policy object.

The compiled path lowers utility selection to typed candidates plus ordinary
condition checks and greatest-score comparison. Standalone utility has no
persistent controller state. The separate flow-only `when policy` form retains
its existing hysteresis and minimum-commit state.

## Evidence-fitted linear utility

The `Statistics` package owns one bounded continuous-target model:

```oct
let options = Statistics.LinearUtilityFitOptions {
    FeatureNames: ["Latency", "AccuracyRisk", "Memory", "Confidence"]
    FeatureSigns: [
        Statistics.UtilityFeatureSign.Nonpositive,
        Statistics.UtilityFeatureSign.Nonpositive,
        Statistics.UtilityFeatureSign.Nonpositive,
        Statistics.UtilityFeatureSign.Nonnegative,
    ]
    IncludeBias: true
    Normalize: true
    L2Regularization: 0.000001
    MaximumLeaveOneOutWeightDelta: 0.25
    IdentificationEvidenceHash: "sha256:..."
    HeldOutEvidenceHash: "sha256:..."
}

let fit = Statistics.FitLinearUtility(observations, options)?
let floatScore = Statistics.ScoreLinearUtility(
    Statistics.UtilityFeatureVector {
        Names: options.FeatureNames
        Values: [latency, risk, memory, confidence]
    },
    fit,
)?
let utilityScore = Statistics.QuantizeLinearUtilityScore(floatScore, 1000.0)?
```

`UtilityObservations` is a `record table` with `CaseName`, `Features`,
`Target`, `Importance`, and `IsHeldOut` columns. Each feature vector must have
the declared schema length. Feature count is bounded to 1 through 32 and the
leave-one-out implementation bounds evidence to 256 observations. Names
must be nonempty and unique, evidence must be finite, importance must be
positive, and identification evidence must be nonempty. Exact names and source
order are stored in the model and checked again at scoring, so a reordered
Accuracy/Latency vector is rejected.

The implementation fits weighted ridge normal equations and solves the bounded
system with LinearAlgebra's deterministic pivoted LU solver. Bias is optional
and is not regularized. Unregularized underdetermined and singular systems are
rejected; ridge regularization may resolve rank deficiency. This first version
does not claim a numerical condition number. It reports a conditioning-or-
stability warning from deterministic leave-one-identification-row-out
coefficient sensitivity.

Normalization means and population scales are fit using identification rows
only, then applied unchanged to held-out evidence. Constant identification
features are rejected when normalization is enabled. The immutable model stores
feature names, weights, bias, normalization, regularization, algorithm version,
evidence hashes, and a readable canonical identity containing those values.
Octagon therefore serializes the coefficients as ordinary fields rather than
an opaque blob.

Training and held-out metrics report row count, weighted MSE, weighted MAE,
maximum absolute error, and R-squared when the target has nonzero variance.
Certification requires at least one held-out row, valid requested coefficient
signs, and leave-one-out stability. Zero held-out rows always produce an
uncertified model. Sign constraints validate and reject authority at the model
boundary; fitting never silently clamps a coefficient.

`ScoreLinearUtility` returns `Float`. Existing `when utility` continues to
accept dimensionless `Int`, so `QuantizeLinearUtilityScore(score, scale)` is the
explicit bridge. It rejects nonfinite score/scale values and uses Oct's named
round-to-nearest operation; no learned score is silently truncated. The
ordinary score breakdown API exposes each coefficient contribution. Model
comparison evaluates an authored or uniform model on the same held-out rows.

Pairwise-preference fitting, constrained optimization, robust scaling,
nonlinear models, automatic feature discovery, online updates, and generalized
cross-validation are deferred. They are not hidden behind this API.

Stable utility-fitting error prefixes are:

- `OCT-UFIT001`: invalid feature-name schema or feature-count bound;
- `OCT-UFIT002`: invalid fit options or sign-schema length;
- `OCT-UFIT003`: empty, oversized, mismatched, nonfinite, or nonpositive evidence;
- `OCT-UFIT004`: insufficient, singular, or numerically unresolved identification system;
- `OCT-UFIT005`: zero-range or invalid identification normalization;
- `OCT-UFIT006`: scoring schema/order/count or finite-value failure;
- `OCT-UFIT007`: invalid or out-of-range Float-to-Int quantization;
- `OCT-UFIT008`: nonfinite fitted coefficient.

## `record table`

```oct
record table Measurements {
    Stage: String
    Latency: Float
    Error: Float
}

let results = Measurements {
    Stage: ["Attention", "RMSNorm", "FFN"]
    Latency: [2.4, 0.008, 0.96]
    Error: [0.001, 0.00002, 0.035]
}
```

Schema field types are cell types. The value stores `Stage: String[]`,
`Latency: Float[]`, and `Error: Float[]` in declaration order. Array-valued
cell types are unambiguous: `Samples: Float[]` means `Float[][]` column
storage. A schema must have at least one unique column.

Construction is complete and exact. Statically visible array literal lengths
are compared during type checking. Dynamic arrays receive one construction
check; disagreement raises `OCT-RTBL003` before a value exists. After
construction, `Len(table)` is the shared row count and does not revalidate.

Column access returns the typed storage array. `table[i]` bounds-checks and
copies one immutable compiler-owned row record with the schema's cell fields.
The row identity is internal and does not need a user declaration. Functions
may accept and return table values, and tables may be fields of ordinary
records.

`for row in table` is deferred because Oct's current `for` syntax is explicitly
range-based. Use `for i in 0..Len(table) { let row = table[i] }`. Table `with`,
mutation, append/delete, table-valued cells, SQL, joins, group-by, reflection,
and dataframe query planning are excluded.

Diagnostics introduced by this detour are:

- `OCT-RTBL001`: empty table schema;
- `OCT-RTBL002`: inconsistent statically known literal lengths;
- `OCT-RTBL003`: inconsistent dynamic lengths at construction;
- `OCT-RTBL004`: immutable table update through `with`;
- `OCT-RTBL005`: row bounds failure.
- `OCT-RTBL006`: unsupported table-valued cell type.
- `OCT-RTBL007`: duplicate table column;
- `OCT-RTBL008`: missing table column;
- `OCT-RTBL009`: extra table column;
- `OCT-RTBL010`: wrong column storage/element type.

## Prometheus M49/M49a lab dogfood

The numerical-heterogeneity M0 lab keeps its established `when utility` syntax.
Its hand-maintained parallel `Depths` and `Errors` arrays are a validated
`record table M0Trace`. It now projects 48 trajectory observations into
`Statistics.UtilityObservations`, fits five coefficients on 24 identification
rows, evaluates without refitting on 24 synthetic held-out rows, and uses the
explicitly quantized learned score in the existing mitigation choice. The
artifact reports learned, authored, and uniform selections and held-out errors.
This remains synthetic design evidence with zero product authority.

M1 separately projects seven clean native RTX 3070 identification records into
the same table API and fits a three-feature shadow model. It compares learned,
authored, and uniform path orderings through existing `when utility`. Because
the native artifact contains zero held-out rows, its model is explicitly
uncertified, held-out baseline improvement is unavailable, and its selection is
shadow-only. M49a resumes at the audit-only matched-input hardware expansion:
collect held-out path/stage/shape/family evidence before any certification or
product authority can change.
