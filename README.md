# CLI

CLI Interface to the Lekko Dynamic Configuration Platform.

## Documentation 

Find our documentation at https://app.lekko.com/docs.

## Download

To download `lekko` cli, you can use homebrew. Since the code is still in a private repository, you will need to use a Github personal access token that has been given access to `lekkodev/homebrew-lekko` and `lekkodev/cli` repos.

```bash
export HOMEBREW_GITHUB_API_TOKEN=<MY_GITHUB_TOKEN>
brew tap lekkodev/lekko
brew install lekko
```

## Release

We use `goreleaser` (https://goreleaser.com/) to compile and release new versions of the lekko cli.

To install:

```bash
brew install goreleaser
```

In order to release a new version of `lekko`, first create a new tag.

```bash
git tag -a v0.1.0 -m "First release"
git push origin v0.1.0
```

Then, export your Github access token to an environment variable. You can create a new GitHub token (here)[https://github.com/settings/tokens/new].

```bash
export GITHUB_TOKEN="YOUR_GH_TOKEN"
```

Finally, create the release.

```bash
goreleaser release
```

That will cross-compile the binary for multiple platforms and architectures, using the latest tag found on github.

After completion, navigate to https://github.com/lekkodev/cli/releases/ to see the latest releases under the tag you just created.

Done! The cli has just been released. Follow instructions above to [Download](#download) the latest cli.

## Integration Tests

Integration tests exist for the cli and they are run on CI by default. You can run them locally:

```bash
GITHUB_TOKEN=${github_personal_access_token} make integration
```

The tests are configured to use the [lekkodev/integration-test](https://github.com/lekkodev/integration-test) repository as a remote for testing.

All tests live in `./pkg/repo/repo_integration_test.go`. They aren't run via `make test` due to the use of a [Go Build Constraint](https://pkg.go.dev/go/build#hdr-Build_Constraints).
