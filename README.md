# codist-git

How [Codist](https://github.com/AbiXnash/codist) reads a repository: commits, changed lines, files at any commit,
blame, branches and churn, through the `git` command. Everything goes through `Repo`, so a pure-Go backend can
replace the command where git is not installed.

```go
import git "github.com/AbiXnash/codist-git"

repo, err := git.Open(".")
log, _ := repo.Log(50)
changes, _ := repo.Changes(log[0].SHA) // files, added line numbers, patches
```

`testrepo` builds a small Spring repository as a real git repository for tests: order-service calls
payment-service through Feign, one commit adds a bug and the next fixes it, and a branch is one commit ahead.

```bash
go test ./...
```

## Licence

Source available under the [PolyForm Noncommercial License 1.0.0](LICENSE): free for any noncommercial purpose, such
as personal study, research, hobby projects and testing, and for use by charities, schools, public research and
public health bodies and government institutions. **Using it for a business, or in anything sold, needs a commercial
licence** from the author: open an issue at [github.com/AbiXnash/codist](https://github.com/AbiXnash/codist/issues)
to ask. Third-party code included in Codist keeps its own licence.

Required Notice: Copyright 2026 Abinash (https://github.com/AbiXnash)
