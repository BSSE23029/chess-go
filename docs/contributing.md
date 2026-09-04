# Contributing

Keep chess rules independent of terminal rendering, transport, and deployment
configuration. New protocol behavior must validate moves and synchronization on
the authoritative server, not only in a client.

Before opening a change, run:

```console
make verify
```

Add focused tests for every behavior change. Keep maintained Go source below the
500-line hard gate (300 lines is the soft target); split a file when a feature
would cross that boundary. Use environment variables or explicit flags for
machine-specific paths, names, tokens, and external engine commands.

Commits should describe one coherent verified cycle. Do not commit generated
`dist/`, PGN, or JSON output. No remote push is implied by local development.
