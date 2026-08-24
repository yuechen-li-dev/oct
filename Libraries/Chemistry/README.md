# Chemistry

This package complements `ChemistryNmr`: NMR retains spectral construction/analysis, while `Chemistry` owns foundational solution relations and bounded kinetics.

[`ChemistryNmr`](../ChemistryNmr/README.md) owns NMR-specific spectra and shifts; [`Units.Spectroscopy`](../Units/README.md) owns reusable presentation/conversion records such as ppm and wavenumber. Generic fitting and summaries belong to [`Statistics`](../Statistics/README.md). Start with `SolutionStateAt`, dilution, or the kinetics facts in `Chemistry.Core.octest`.

The solution chapter covers molarity, dilution, ideal concentration-based pH, Henderson-Hasselbalch, and Beer-Lambert absorbance. `SolutionState` is a record-shaped Concept tying amount, volume, and concentration together.

The kinetics chapter covers first-order progress/half-life, Arrhenius temperature dependence, Michaelis-Menten rate, and Hill occupancy. `ReactionProgress` keeps amount and converted fraction inspectable. These are textbook models: pH neglects activity coefficients, Beer-Lambert assumes homogeneous non-scattering samples, Michaelis-Menten uses the standard single-substrate quasi-steady assumptions, and Hill occupancy is phenomenological.
