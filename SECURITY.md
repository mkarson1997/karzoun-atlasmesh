# Security Policy

## Supported versions

Security fixes are applied to the latest released minor version and `main`.

## Reporting a vulnerability

Please use GitHub's **Private vulnerability reporting** for this repository when available. Do not open a public issue for a suspected vulnerability.

Include the affected component, reproduction conditions, realistic impact and a minimal proof of concept. Do not include production credentials, private infrastructure addresses or unrelated personal data.

## Security model

AtlasMesh v0.1 is a control-plane foundation, not an internet-ready multi-tenant authorization server. Deploy it only behind an authenticated trusted network boundary until the authentication/authorization milestone lands.

The project treats stale lease ownership, unbounded input, SSRF-capable health targets, secret leakage in logs and dependency supply-chain risk as security concerns.
