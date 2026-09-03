---
name: environment-config
description: Keep user-, machine-, secret-, and deployment-specific runtime values out of source code by resolving and validating them from environment variables. Use when adding configuration, identity, credentials, endpoints, paths, ports, or deployment settings.
---

# Environment-backed configuration

Do not hard-code values that vary by user, machine, account, secret, environment, or deployment.

- Read each runtime value from a clearly named environment variable at the application boundary.
- Validate required values immediately and return an actionable error naming the variable.
- Use a default only when it is safe and genuinely universal; keep secrets required.
- Pass resolved configuration into the code that needs it so tests can supply values without mutating process-wide state.
- Use the standard library for environment access. Do not add a dotenv or configuration dependency unless requested.

Do not move stable domain rules, protocol constants, or compile-time language identifiers into environment variables. Go module and import paths are compile-time string literals and cannot interpolate environment variables; use a deliberately chosen stable module identity instead.
