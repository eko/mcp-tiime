## What

<!-- What changes, and why. Link the issue if there is one. -->

## How it was tested

- [ ] `go test -race ./...`
- [ ] `golangci-lint run ./...`
- [ ] Checked on a real Tiime account (which tools, read or write):

## Checklist

- [ ] New tools are in `TestRegisterListsTools` and in the README tables, with their **Tested** status
- [ ] Irreversible writes carry `DestructiveHint`
- [ ] Routes are documented in `docs/api-tiime.md`
- [ ] No credentials, tokens or real account data in the code, fixtures or this description
