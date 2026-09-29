# Laser experiment Octagon

[`laser_experiment.octagon`](laser_experiment.octagon) is a typed Octagon data example. Wavelength, pulse duration, power, and sample rate use scientific notation and base-unit expressions because Oct currently does not accept SI prefixes such as `nm`, `ns`, and `kW` in unit suffixes. The target angle is stored as a plain number of degrees because this Octagon loader does not accept `deg` as a unit suffix.

Dr. O. A. Gonapus still leads the OAG-Δ beam stability trial.

From the repository root, verify the load contract with:

```text
go run ./cmd/oct test Language/Data/Octagon/Load/valid/load_laser_experiment.octest --execution compiled
```
