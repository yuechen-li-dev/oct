# Quantum

This deliberately small chapter moves through photon energy, matter wavelength, ideal discrete energy levels, uncertainty products, and finite-state Born probabilities.

`QuantumLevel` is a refined positive-integer Concept. Runtime levels use `AdmitQuantumLevel`. `DiscreteProbabilityState` is a record-shaped Concept that keeps normalized probabilities with the original norm, making normalization inspectable rather than magical.

`ParticleInBoxEnergy` is the infinite one-dimensional well. `HydrogenEnergyLevel` is the non-relativistic Z=1 reference formula with the usual infinite-nuclear-mass approximation. `DeBroglieWavelength` uses non-relativistic momentum `m*v`. The finite-state helpers normalize small explicit amplitude arrays and do not constitute a Schrödinger solver or quantum-computing framework.
