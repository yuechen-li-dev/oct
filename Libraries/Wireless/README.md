# Wireless

`Wireless.Typed` is the preferred modern surface. Frequencies/bandwidths use `Float<Hz>`, power uses `Float<kg*m^2/s^3>`, temperature uses `Float<K>`, and throughput uses `Float<Hz>` because bit counts are dimensionless.

The original scalar-Hz records/functions remain for source compatibility. Their historical `TxPowerW` type also carries an extra inverse-current-squared dimension and therefore cannot be silently changed. New work should use `WirelessBandSI`, `WirelessLinkBudgetSI`, `WirelessThroughputSI`, and the `*SI` functions.

Free-space path loss assumes unobstructed far-field propagation. Thermal noise is the ideal `k*T*B` model. Shannon spectral efficiency remains capped by the package's historical 10-bit/s/Hz implementation assumption. These are reference link-budget relations, not regulatory channel data or a propagation simulator.
