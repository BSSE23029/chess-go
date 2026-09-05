# Network and authenticator security

This project separates transport confidentiality, protocol compatibility, and
user authentication. None of those layers can substitute for the others.

## Transport defaults

`chess host` requires a certificate and private key by default and configures a
minimum TLS version of 1.3 for HTTP and WebSocket traffic. Set
`CHESS_TLS_CERT` and `CHESS_TLS_KEY` (or the matching flags) at the deployment
boundary. Clients use the operating-system trust store; `CHESS_TLS_CA` can add a
private CA, and `CHESS_TLS_CLIENT_CERT`/`CHESS_TLS_CLIENT_KEY` enable mutual TLS.

`--insecure` and `CHESS_NETWORK_INSECURE=true` are explicit exceptions for an
isolated development machine. They are not authentication and must not be used
on an untrusted LAN or the public internet. Bearer tokens still need high
entropy, rotation, and secret-manager storage even when TLS is enabled.

The Go TLS implementation follows the TLS 1.3 protocol defined by
[RFC 8446](https://www.rfc-editor.org/rfc/rfc8446). The server does not disable
certificate verification or install an “accept any certificate” client mode.

## Protobuf migration

The checked-in `protocol/pb/envelope.proto` defines the binary envelope. Its
payload is intentionally still the existing strict JSON object: this permits a
wire-format migration without duplicating authoritative match validation. Use
`CHESS_NETWORK_FORMAT=protobuf` for the environment-backed client constructor.
Unknown protobuf fields remain forward-compatible, while JSON payload fields
remain strict. When typed protobuf payloads are introduced, follow the official
[field-number evolution rules](https://protobuf.dev/best-practices/dos-donts/):
never reuse deleted tags, reserve them, and avoid required fields.

## TOTP authenticator

`chess-go/auth` implements the RFC 6238 time-based one-time password profile
with an eight-digit code, a 30-second timestep, a 160-bit random secret, and a
maximum ±1-step clock window. `auth.Verifier` records the highest accepted
timestep per account and rejects replay. Enrolled secrets can be wrapped with
AES-256-GCM; the encryption key must come from a deployment secret manager or
KMS and is never generated from a username or checked into source.

TOTP must be provisioned only over the authenticated TLS channel. The
`otpauth://` URI contains the secret and must be treated as a credential, not
as an ordinary loggable URL. RFC 6238 recommends a small validation window and
rejecting reuse within a timestep; this implementation follows that guidance:
[RFC 6238](https://www.rfc-editor.org/rfc/rfc6238).

TOTP is not phishing-resistant because a user manually types a short-lived
code into a potentially malicious page. NIST explicitly makes that distinction
in [SP 800-63B](https://pages.nist.gov/800-63-4/sp800-63b.html). For a public
service, use WebAuthn/passkeys as the primary phishing-resistant factor and
keep TOTP as a recovery or compatibility factor. The WebAuthn model keeps a
private key in the authenticator and scopes credentials to the relying party:
[WebAuthn Level 3](https://www.w3.org/TR/webauthn-3/).

## Deployment checklist

- Use a real certificate chain and verify its hostname from clients.
- Keep `CHESS_NETWORK_TOKEN`, TOTP secrets, and AES keys outside source control.
- Rotate bearer tokens and client certificates; revoke lost authenticators.
- Persist replay state if authentication must survive a process restart.
- Rate-limit failed codes at the account and network boundary.
- Prefer passkeys for new accounts; never describe an eight-digit TOTP as
  phishing-proof or universally “top research level safe.”
