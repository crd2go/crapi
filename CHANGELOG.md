# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## 2026/07/17

### Added

- Field-level transformer registry for `Translator`, allowing callers to register custom conversion logic for specific fields when the default JSON round-trip translation (map[string]any) does not produce a correct or stable result (e.g. CRD `*string` date-time fields vs. SDK `*time.Time` fields).
  - New `FieldTransformer` type: `func(fieldPath string, value any) (any, error)`.
  - New sentinel error `ErrNoMatch`, returned by a `FieldTransformer` to indicate it does not apply to the given field path/value, allowing the next registered transformer (if any) to be tried.
  - New `TranslatorOption` type and two option constructors:
    - `WithToAPIFieldTransformer(fieldPathPattern string, fn FieldTransformer) TranslatorOption` — registers fn to run during `Translator.ToAPI` for any field path matching the anchored regular expression `fieldPathPattern`.
    - `WithFromAPIFieldTransformer(fieldPathPattern string, fn FieldTransformer) TranslatorOption` — same, but for `Translator.FromAPI`.
  - Transformers are applied during a single recursive walk of the translated object map, matching only against leaf values (not containers); the first registered transformer for a matching path that does not return `ErrNoMatch` "wins" for that field.
  - `NewTranslator` now accepts optional trailing `opts ...TranslatorOption` (backwards compatible — existing 4-argument calls are unaffected).

### Changed

- **BREAKING:** `NewPerVersionTranslators`'s `versions` parameter changed from variadic (`versions ...string`) to a slice (`versions []string`), to make room for the new trailing `opts ...TranslatorOption` parameter (a function may only have one variadic parameter, and it must be last). Existing callers must update call sites, e.g.:

  ```diff
  - NewPerVersionTranslators(scheme, crd, "v1", "v20250312", "v20250810")
  + NewPerVersionTranslators(scheme, crd, "v1", []string{"v20250312", "v20250810"})
  ```

  `NewTranslator` is unaffected by this and remains fully backwards compatible.

