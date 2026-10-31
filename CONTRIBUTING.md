# Contributing to Auteur

Thanks for considering a contribution. Auteur is small and early —
issues and PRs are welcome.

## Development

    git clone git@github.com:stythelad/auteur.git
    cd auteur
    make test
    make build

You need Go 1.22+ and git on PATH.

## What we're looking for

- Bug reports with a minimal reproduction.
- New agent detections in internal/heuristic/trailers.go.
- Policy rule types that catch a real class of risk.
- Editor integrations.

## What we're not looking for (yet)

- New scanner engines. Auteur does attribution, not vulnerability detection.
- Dashboards, web UIs, telemetry.
- Rewrites of the ledger format.

## Code style

- gofmt -w . before committing.
- Tests for anything with non-trivial logic.
- No new dependencies without a reason in the PR description.

## Pull requests

- One logical change per PR.
- go test ./... must pass.
- If you're touching a CLI surface, update README.md.

## License

By contributing, you agree your contribution is licensed under MIT.
