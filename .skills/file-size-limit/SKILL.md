---
name: file-size-limit
description: Enforce this Go repository's 300-line target and 500-line hard ceiling for .rs files. Use when adding to a Go file near 300 lines, when the checker reports a breach, or when deciding how to split an oversized Go module without changing behavior.
---

# Go file size: 300 soft, 500 hard

Count physical lines with `wc -l`:

- 300 lines is the target. Crossing it prompts a search for a clean module seam.
- 500 lines is the hard ceiling. No maintained `.rs` source file may exceed it.

Run:

```bash
standards/Engineering/file-size-limit/check.sh
standards/Engineering/file-size-limit/check.sh lib/path/to/file.rs
standards/Engineering/file-size-limit/check.sh --hard
standards/Engineering/file-size-limit/check-folders.sh
```

The checker scans Go source in the repository and excludes build output under `target/`.

## Splitting Go safely

Read the full module before splitting it. Move complete declarations with their documentation and
tests. Prefer normal Go modules (`foo.rs` plus `foo/`) and preserve the existing public import
path with `mod` declarations and `pub use` only when callers need it.

A split is a pure move. Do not combine it with behavior changes, visibility widening, reordered
side effects, or removed documentation. Do not introduce a module solely to pad tiny files.

When a file approaches 250 lines, place the next coherent concern in a submodule. Natural seams
include entity types, SQL/migration helpers, validation, authorization resolution, and tests.

## Verification

After a split:

```bash
cargo fmt --all -- --check
cargo check --workspace
standards/Engineering/file-size-limit/check.sh --hard
```

Run the focused tests for the moved behavior. Review the diff to ensure the split did not change
logic.

Keep at most five immediate files in every maintained project folder. When a folder would exceed
five files, introduce a cohesive feature subfolder and preserve public Go paths with `#[path]`,
`mod`, and `pub use` as appropriate. Verify with `check-folders.sh` from the repository root.
