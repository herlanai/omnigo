# Validation scripts

Run the framework checks from the repository root:

```sh
sh scripts/check.sh
```

`check.sh` runs Go tests, vet, race detection, TypeScript SDK tests, desktop
cross-compilation, and the lifecycle example. `ci/` contains the browser-project
MJS adapter and the macOS visible-window Swift probe used by GitHub Actions.

Go development commands, inventory audits, native CI verification, and process
sampling live in [tools/cmd](../tools/README.md). Studio pnpm shortcuts remain
available at the repository root.
