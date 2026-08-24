# Units

## Shelf boundary

The compiler owns canonical SI dimensions. `Units` owns explicit non-SI and domain presentation records plus conversion into SI: `Units.American`, `Units.British`, `Units.Astronomical`, `Units.Atomic`, and `Units.Spectroscopy`. It does not own physical constants or equations; use [`Physics`](../Physics/README.md) for those. Start with `Units.Core.octest` for SI-safe helpers or the matching family test for a conversion.

Oct's compiler owns SI dimensions: `m`, `kg`, `s`, `A`, `K`, `mol`, `cd`, and the named `Hz` alias. Derived quantities remain ordinary expressions such as `kg*m/s^2`, `kg/m/s^2`, `A*s`, and `kg*m^2/(A*s^3)`; this keeps dimensional arithmetic visible and statically checked.

The `Units` package owns legitimate non-SI presentation/domain records and explicit conversion to SI:

- American and British customary length, mass, volume, temperature, pressure, energy, and power
- astronomical distances, masses, luminosity, flux density, and times
- atomic length, energy, mass, time, and cross-section units
- spectroscopy ratios, wavenumber, concentrations, and mass-spectrometry units

Conversions distinguish exact definitions in comments (inch, pound, AU, US volume relationships, electron volt) from nominal or measured conventional values (solar quantities, particle masses, Hartree). New M1 culinary additions include typed US tablespoons and teaspoons. There are no string-parsed unit conversions and no competing derived-unit type system.
