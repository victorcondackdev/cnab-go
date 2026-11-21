# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project
adheres to [Semantic Versioning](https://semver.org/).

## [0.1.0] - 2026-11-21

### Added

- `fixedwidth`: declarative struct-tag codec (`num`, `alfa`, `raw`, `date`,
  `money`, `const`) with layout self-validation (gaps, overlaps, out-of-range
  positions, type mismatches) cached per type, and positional `FieldError`s.
- `cnab240`: FEBRABAN CNAB 240 cobrança records: file header/trailer, batch
  header/trailer, segments P, Q, R (remessa) and T, U (retorno); raw
  passthrough for other segments; movement, espécie and instruction codes.
- `cnab400`: generic CNAB 400 header/detail/trailer skeleton and an
  experimental Bradesco remessa detail layout.
- `file`: `Parse`/`Write` with structural validation and automatic
  regeneration of counts and sequences, `Parse400`/`Write400`, format
  detection, and the `NewRemessa` builder with CPF/CNPJ validation.
- `reconcile`: retorno-versus-expected reconciliation with explained
  differences and a text summary.
- `cmd/cnab`: `inspect`, `validate` and `reconcile` commands with `--json`.
