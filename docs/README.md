# CPA Orbit documentation

This directory is the canonical home for project documentation. The repository root intentionally keeps only the public entry point (`README.md`), license, and machine-readable project metadata.

## Architecture

- [System architecture and high-level diagrams](architecture/README.md)
- [Architecture decision records](architecture/adr/)

## Development

- [Desktop host, packaging, and data migration](development/desktop.md)
- [Backend API, runtime storage, and security boundaries](development/backend.md)
- [Design and implementation plans](development/plans/)

## Releases

- [Changelog](releases/CHANGELOG.md)
- [v1.0.2 release notes](releases/v1.0.2.md)

## Community

- [Community handbook](community/README.md)
- [Contributing guide](CONTRIBUTING.md)
- [Code of Conduct](CODE_OF_CONDUCT.md)
- [Security policy](SECURITY.md)
- [Support policy](SUPPORT.md)
- [Third-party notices](THIRD_PARTY_NOTICES.md)

## Assets

- `assets/showcase/` contains sanitized public product screenshots used by the root README.
- Runtime credentials, imported subscription JSON, local settings, logs, and private verification captures are intentionally excluded from source control.

The in-app operating guide is available at `http://127.0.0.1:5173/docs` while the development server or desktop application is running.
