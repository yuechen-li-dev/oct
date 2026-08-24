# Computational Biology

## Shelf boundary

`ComputationalBiology` owns biological alphabets, sequence operations, translation, consensus, and bounded population models. Generic search stays in [`Algorithms`](../Algorithms/README.md), and generic summaries/regression stay in [`Statistics`](../Statistics/README.md). Start with the typed DNA example below; render only at an external text boundary.

Canonical bases are `DNABase` and `RNABase` enums. Exhaustive `match` expressions own complement, transcription, and symbol rendering, so adding an alphabet variant forces every transform to be reconsidered. Readable `String[]` input remains a validated bridge through `ParseDNA`/`RenderDNA`. Consensus uses enum-targeted `when utility` to score A/C/G/T candidates with a visible deterministic tie order; equal scores choose the earliest listed candidate. `BaseComposition` and `GenotypeFrequencies` are record-shaped Concepts because their fields jointly describe scientific domain values.

The sequence chapter includes DNA/RNA validation, composition and GC content, reverse complement, transcription, Hamming distance, identity, consensus, overlapping query k-mers, the standard nuclear genetic code, and in-frame translation. `TranscribeTypedDNA` treats its input as the 5'-to-3' coding strand (T becomes U); a template strand needs complement/orientation handling first. `TranslateTypedDNA` keeps enum-based callers typed through translation. Complexity is stated on the algorithms where scale matters; these are small-sequence educational utilities, not a database or production aligner.

```oct
import ComputationalBiology

let coding = [
    ComputationalBiology.DNABase.Adenine,
    ComputationalBiology.DNABase.Thymine,
    ComputationalBiology.DNABase.Guanine
]
let rna = ComputationalBiology.TranscribeTypedDNA(coding)?
let translation = ComputationalBiology.TranslateTypedDNA(coding)?
// ATG becomes AUG and translates to methionine ("M").
```

`"Stop"` is an explicit termination token, not an amino acid. Package-local facts use `!` for known-good cases; reusable consumers normally propagate with `?`.

The population chapter adds Hardy-Weinberg frequencies plus exponential and logistic growth. Hardy-Weinberg assumes the textbook equilibrium conditions; logistic growth assumes constant rate and carrying capacity. Stop codons are returned explicitly as `"Stop"` rather than silently ending translation.
