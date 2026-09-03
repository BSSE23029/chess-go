---
name: progress-commits
description: Create small verified Git progress commits for repository implementation work while preserving user changes and prohibiting AI attribution. Use after each completed and tested implementation cycle.
---

# Progress commits

After each coherent implementation cycle:

1. Inspect `git status` and the diff. Treat unrelated or pre-existing changes as user-owned.
2. Run the focused tests and required repository gates.
3. Stage only explicit files belonging to the completed cycle; never stage broad paths blindly.
4. Commit with a concise human project message describing the outcome.
5. Verify the commit contents and leave unrelated work untouched.

Never add AI attribution anywhere. This includes:

- `Co-authored-by` or similar AI trailers.
- AI names in commit authors, committers, contributors, acknowledgements, changelogs, manifests,
  generated metadata, pull-request text, or repository documentation.
- Automated edits to contributor lists.

Use the repository's configured human Git identity unchanged. Do not rewrite authorship. Do not
amend, rebase, force-push, or push to a remote unless the user explicitly requests that separate
operation. A failed verification means the cycle is not ready to commit.
