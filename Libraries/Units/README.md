# Units

Oct's compiler owns SI dimensions: `m`, `kg`, `s`, `A`, `K`, `mol`, `cd`, and the named `Hz` alias. Derived quantities remain ordinary expressions such as `kg*m/s^2`, `kg/m/s^2`, `A*s`, and `kg*m^2/(A*s^3)`; this keeps dimensional arithmetic visible and statically checked.

The `Units` package owns legitimate non-SI presentation/domain records and explicit conversion to SI:

- American and British customary length, mass, volume, temperature, pressure, energy, and power
- astronomical distances, masses, luminosity, flux density, and times
- atomic length, energy, mass, time, and cross-section units
- spectroscopy ratios, wavenumber, concentrations, and mass-spectrometry units

Conversions distinguish exact definitions in comments (inch, pound, AU, US volume relationships, electron volt) from nominal or measured conventional values (solar quantities, particle masses, Hartree). New M1 culinary additions include typed US tablespoons and teaspoons. There are no string-parsed unit conversions and no competing derived-unit type system.
