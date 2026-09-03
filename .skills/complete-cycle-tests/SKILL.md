---
name: complete-cycle-tests
description: Require complete lifecycle and authorization tests for every Go behavior added or changed in this repository. Use for all implementation, bug-fix, migration, service, handler, and security work.
---

# Complete-cycle tests

Implementation is incomplete until its externally meaningful cycle is tested.

For each changed behavior, cover every applicable phase:

1. Create or initialize.
2. Read through the real public/service boundary.
3. Update or transition state.
4. Reject invalid input and unauthorized cross-tenant/object access.
5. Soft-delete or deactivate, verify normal reads exclude it, then restore when supported.
6. Verify durable side effects such as audit actor/timestamps, permission snapshots, tokens, or
   lifecycle state.

Use the smallest test level that proves the whole behavior. Database and authorization behavior
requires PostgreSQL integration tests; a unit test with mocks is not sufficient evidence. Reuse
existing fixtures and SeaORM services. Do not duplicate production logic inside assertions.

Migrations require both paths:

- Fresh database: all migrations apply and produce the expected schema and constraints.
- Upgrade database: representative pre-migration data survives and is backfilled correctly.

Run focused tests while iterating, then `cargo test --workspace` before calling a cycle complete.
Record any environment-dependent test that cannot run; never silently substitute compilation for
behavioral verification.
