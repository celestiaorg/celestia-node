# Contributing

Thank you for your interest in contributing to Celestia-Node! 

All work on the code base should be motivated by [our Github
issues](https://github.com/celestiaorg/celestia-node/issues). If you
would like to work on an issue which already exists, please indicate so
by leaving a comment.

All new contributions should start with a [Github
issue](https://github.com/celestiaorg/celestia-node/issues/new/choose). The
issue helps capture the problem you're trying to solve and allows for
early feedback. Once the issue is created the process can proceed in different
directions depending on how well-defined the problem and potential
solution are. If the change is simple and well understood, maintainers
will indicate their support with a heartfelt emoji.

When the problem is well understood but the solution leads to large structural
changes to the code base, these changes should be proposed in the form of an
[Architectural Decision Record (ADR)](./docs/adr/). The ADR will help
build consensus on an overall strategy to ensure the code base maintains
coherence in the larger context. If you are not comfortable with writing an
ADR, you can open a less-formal issue and the maintainers will help you turn it
into an ADR.

> How to pick a number for the ADR?

Find the largest existing ADR number and bump it by 1.

Each stage of the process is aimed at creating feedback cycles which align contributors and maintainers to make sure:

- Contributors don’t waste their time implementing/proposing features which won’t land in master.
- Maintainers have the necessary context in order to support and review contributions.

## PR Naming

PRs should be titled as following: 
```txt
pkg: Concise title of PR
```

**Example:**

```txt
service/header: Remove race in core_listener
```

## Changelog

Every *notable* fix, improvement, feature, or breaking change should be made in a
pull-request that includes an update to the `CHANGELOG_PENDING.md` file.

Changelog entries should be formatted as follows:

```md
- [module/pkg: Some description about the change #xxx](link to PR) [@contributor](link to contributer github) 
```

Here, `module` is the part of the code that changed (typically a
top-level Go package), `xxx` is the pull-request number, and `contributor`
is the author/s of the change.

## Branching Model and Release

The main development branch is `main`.

Every release is maintained in a release branch named `vX.Y`. On each respective release branch, we tag the releases 
vX.Y.0, vX.Y.1 and so forth.

Note all pull requests should be squash merged except for merging to a release branch (named `vX.Y`). This keeps the commit history clean and makes it
easy to reference the pull request where a change was introduced.

### Development Procedure

The latest state of development is on `main`, which must never fail `make test`. _Never_ force push `main`.

To begin contributing, create a development branch on your fork.

Make changes, and before submitting a pull request, update the `CHANGELOG_PENDING.md` to record your change. Also, `git 
rebase` on top of the latest `main`. 

Sometimes (often!) pull requests get out-of-date with main, as other people merge different pull requests to main. It is
our convention that pull request authors are responsible for updating their branches with `main`. (This also means that you shouldn't update someone else's branch for them; even if it seems like you're doing them a favor, you may be interfering with their git flow in some way!)

#### Merging Pull Requests

It is also our convention that authors merge their own pull requests, when possible. External contributors may not have the necessary permissions to do this, in which case, a member of the core team will merge the pull request once it's been approved.

Before merging a pull request:

- Ensure pull branch is up-to-date with a recent `main` (GitHub won't let you merge without this!)
- Run `make test` to ensure that all tests pass
- Ensure that all other CI checks pass / are green
- [Squash](https://stackoverflow.com/questions/5189560/squash-my-last-x-commits-together-using-git) merge pull request

### Git Commit Style

We follow the [Go style guide on commit messages](https://tip.golang.org/doc/contribute.html#commit_messages). Write concise commits that start with the package name and have a description that finishes the sentence "This change modifies Celestia-Node to...". For example,

```sh
cmd/debug: execute p.Signal only when p is not nil

[potentially longer description in the body]

Fixes #nnnn
```

It is recommended to prepend the type of change the commit is making to the commit message as documented [here](https://www.conventionalcommits.org/en/v1.0.0/).

```txt
feat(service/header): Title of PR.
```

Each PR should have one commit once it lands on `main`; this can be accomplished by using the "squash and merge" button on Github. Be sure to edit your commit message, though!

## Testing

### Unit tests

Unit tests are located in `_test.go` files as directed by [the Go testing
package](https://golang.org/pkg/testing/). If you're adding or removing a
function, please check there's a `TestType_Method` test for it.

Run: `make test`