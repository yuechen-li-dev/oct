# DatabaseTemplateContracts

This companion package contains the compile-time refined Concepts used by `DatabaseTemplates`. Import both packages in a consuming specialization. The explicit import is currently required because an instantiated template retains the package-qualified refinement identity in the consumer package.

The concepts erase to their ordinary `Int` or `String` representation after static admission. They add no runtime checks, registry, wrapper, or allocation for literal/compile-time configuration. Runtime-derived configuration uses `AdmitPositiveCount`, `AdmitPublicationSource`, or `AdmitPublicationVersion`, which expose ordinary fallible Concept construction to consumer packages.
