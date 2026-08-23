# Computational Science

This package is the cross-library chapter: it does not reimplement scientific laws or solvers. It connects `Simulation` to canonical models in `Thermofluids`, `Electromagnetism`, `Astrophysics`, `ComputationalBiology`, and `Units`.

`DecayExperiment` is a record-shaped Concept containing a deterministic fixed-step trace, the exact first-order result, and the error. The same normalized decay equation is compared with typed lumped cooling and RC discharge. Additional mini-lessons recover a one-AU orbital period and bounded logistic population growth.

The current `Simulation` runner remains scalar and fixed-step. Physical examples normalize state for the runner and use typed library functions at scientific boundaries; vector-state or adaptive simulation is deliberately not improvised here.
