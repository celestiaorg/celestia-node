# Required review policy

PRs need independent human approval according to their risk tier, whatever
their base branch.
GitHub's native required approvals enforce it, and one small workflow,
`.github/workflows/auto-approve.yml`, supplies the approval for PRs that need
no human. The policy supplements CI and conversation resolution; it does not
replace them. It matches the policy in celestia-app.
Approvals are not bound to a commit: they persist across later pushes.

## Review requirements

| Title tag | Independent human approvals |
| --- | --- |
| Low | 0 for a trusted author; otherwise 1 |
| Medium | 0 for a trusted author; otherwise 1 |
| High | 1 |

A trusted author is a human with write, maintain, or admin permission, or
Dependabot, whose tier is fixed by the title prefix in `.github/dependabot.yml`.
Other bot-authored PRs (including coding agents and Mergify backports)
and PRs from people without write permission always need one human approval.
The maintainer who prompted an agent may give it.

The tier comes only from the PR title's conventional-commit `[low]`,
`[medium]`, or `[high]` tag, which `lint-pull-request.yml` requires. Reviewers
confirm the chosen tag, as described in `CONTRIBUTING.md`.

## How it is enforced

The `main` ruleset requires one approving review and code-owner review. GitHub
itself then guarantees the rest: the author cannot approve their own PR, drafts
cannot merge, a change request from a reviewer with write access blocks the
merge, and approvals survive pushes because stale approvals are not dismissed.

`auto-approve.yml` approves as a GitHub App when a PR is not a draft, its title
tag is `[low]` or `[medium]`, and its author is trusted. When a PR
stops qualifying (retitled to `[high]`, converted to a draft, or its author
loses write access), the workflow dismisses the App's approval. `[high]` PRs
are never auto-approved, so they always wait for a human.

The workflow runs on `pull_request_target`, which always uses the default
branch's copy of the workflow, so it covers stacked PRs whatever their base. A
30-minute sweep and `workflow_dispatch` reconcile missed events and permission
changes. It never checks out or runs PR code; it reads only the event payload
and the GitHub API.

`CODEOWNERS` assigns `@celestiaorg/protocol` to this policy,
`auto-approve.yml`, `dependabot.yml`, `CODEOWNERS` itself, the agent
instruction files, and the root `CONTRIBUTING.md`. Because the author picks the
tier, code-owner review ensures a `[low]` PR cannot change this policy or agent
behavior without a protocol team approval; the App's approval never satisfies
code-owner review. Other files under `.github/`, including CI workflows, have
no code owner, so the App's approval covers them like any other change.

## Administrator setup

celestia-node reuses the GitHub App that auto-approves celestia-app PRs.

1. Add this repository to the App's installation. The App needs the
   **Pull requests: write** and **Contents: write** repository permissions
   (metadata read is implicit). GitHub counts an App's approval only when the
   App has write access to the repository; with Pull requests alone its
   approvals exist but do not satisfy the ruleset. The workflow still mints a
   token limited to pull requests.
2. Create an `auto-approve` environment whose deployment branches are limited
   to `main`. Store the App's client ID as the `AUTO_APPROVE_CLIENT_ID`
   variable and its private key as the `AUTO_APPROVE_PRIVATE_KEY` secret in
   that environment, not at repository level.
3. Confirm on a test PR that the App's approval counts toward the required
   approval, and that retitling to `[high]` dismisses it.
4. In the `main` ruleset, set required approvals to 1 and enable "Require review
   from Code Owners". Keep "Dismiss stale pull request approvals when new
   commits are pushed" and "Require approval of the most recent reviewable
   push" disabled, keep conversation resolution and the merge queue, and keep
   the `conventional-commit-pr-title` required check.
5. Keep "Allow GitHub Actions to create and approve pull requests" disabled.
   The App, not `GITHUB_TOKEN`, supplies approvals.

The workflow evaluates PRs whatever their base, including release branches,
so a trusted author's `[low]` or `[medium]` backport PR is approved too.
Mergify backport PRs are bot-authored, so they always need a human approval.
