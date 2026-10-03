# OrbitalDecay

Low Earth Orbit (LEO) atmospheric drag and multi-orbit altitude decay simulation exploring Oct's built-in dimensional analysis and scientific types.

## Running the Experiment

### Milestone M0: Baseline Physical Simulation
```sh
oct run Experiments/OrbitalDecay/M0/orbital_decay_m0.oct
oct test Experiments/OrbitalDecay/M0 --execution compiled
```

### Milestone M1: Parametric Templates & Telemetry Analysis
```sh
oct run Experiments/OrbitalDecay/M1/orbital_decay_m1.oct
oct test Experiments/OrbitalDecay/M1 --execution compiled
```

### Run Entire Experiment Suite
```sh
oct test Experiments/OrbitalDecay --execution compiled
```
