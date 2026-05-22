# bbb-graphql-middleware

## Running tests

Run the focused middleware tests:

```sh
go test ./internal/common ./internal/msgpatch
```

Run all packages while skipping Go vet checks:

```sh
go test -vet=off ./...
```

`go test ./...` also runs vet. At the moment it can fail on existing vet diagnostics in unrelated packages, so use `-vet=off` when you need to verify compile/tests only.

Run the JSON patch benchmark:

```sh
go test -run '^$' -bench BenchmarkLongestCommonSubsequenceLargeList -benchmem ./internal/common
```
