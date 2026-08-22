# PackageRegistryDogfood

A tiny PM7 dogfood package for the source-controlled canonical registry.

From this directory in an Oct repository checkout:

```sh
oct pkg registry add oct ../../Registry
oct pkg add Mathematics@0.1.0
oct pkg sync
oct test .
```

`lock.octagon` is optional. To dogfood lockfile-backed sync after adding the dependency, run:

```sh
oct pkg lock
rm -rf .oct/packages
oct pkg sync --locked
oct test .
```
