# Security Policy

## Supported versions

gocldr is pre-1.0. Security fixes are applied to the most recent release and
to `main`. Until 1.0, only the latest release line is supported.

| Version       | Supported          |
| ------------- | ------------------ |
| Latest `0.x`  | :white_check_mark: |
| Older         | :x:                |

## Reporting a vulnerability

Please report suspected vulnerabilities privately. **Do not open a public issue
for a security problem.**

- Preferred: open a private report through GitHub Security Advisories — the
  **Report a vulnerability** button under the repository's **Security** tab.
- Alternatively, email <headcrabogon@gmail.com> with a description and, where
  possible, a minimal reproduction.

You can expect an acknowledgement within a few business days. Once a fix is
ready we will coordinate disclosure and credit reporters who wish to be named.

## Threat model

gocldr is a CLDR formatting library. The classes of issue we consider in scope:

- Inputs that cause excessive CPU or memory use (e.g. pathological locale tags or
  number values that trigger quadratic behavior).
- Panics on input. The formatters are designed to degrade gracefully and must not
  panic; a reproducible panic is a bug we want to hear about.

Locale data is compiled from Unicode CLDR and treated as **trusted input**. User-
supplied locale tags and numeric/date values are the untrusted surface.
