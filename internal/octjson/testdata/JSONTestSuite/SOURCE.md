# JSONTestSuite

`test_parsing/` and `LICENSE` are copied unchanged from
<https://github.com/nst/JSONTestSuite>, commit
`1ef36fa01286573e846ac449e8683f8833c5b26a` (2024-11-22), by Nicolas Seriot,
under the MIT License in this directory.

A file named `y_*` must be accepted by a JSON parser, a file named `n_*` must
be rejected, and a file named `i_*` may be either. `internal/octjson`
states what it does with each `i_*` file in `parse_test.go`.
