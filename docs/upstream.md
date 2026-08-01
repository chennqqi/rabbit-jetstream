# Upstream NATS Server

`upstream/nats-server/` is a Git subtree pinned to the official `nats-io/nats-server` repository. The initial pin is `v2.14.1`.

[`upstream/nats-server.lock.json`](../upstream/nats-server.lock.json) records the official repository, stable tag, peeled commit, Git tree, and local squash commit. `make verify-upstream` rejects checked-in, staged, untracked, or working-tree changes beneath the subtree. `make verify-upstream-online` additionally fetches the tag into a temporary bare repository and proves that its commit and tree match the lock. CI and production release gates run the online form.

The subtree is the message-server core of this distribution. Management APIs, UI, packaging, deployment defaults, and operational extensions belong outside this directory. Direct upstream edits require an architecture decision record explaining why configuration or an external component cannot solve the problem, plus a plan to upstream or continuously rebase the patch.

The release image may apply a narrowly pinned build-time dependency security override without changing subtree files. `packaging/Dockerfile.nats-server` currently raises `golang.org/x/crypto` to `v0.52.0` because the official `v2.14.1` source pin predates fixes for the 2026 SSH vulnerability set. The override is visible as a Docker build argument, must pass upstream and repository integration/fault tests, and must be removed or advanced when a newer official NATS release incorporates an equal or newer dependency. Source behavior patches are not permitted through this mechanism.

## Updating

Start from a clean branch, review the upstream release and upgrade notes, then run:

```bash
git subtree pull \
  --prefix=upstream/nats-server \
  https://github.com/nats-io/nats-server.git \
  <stable-tag> --squash
```

After an update, build the upstream server and run its tests, this repository's test suites, three-node recovery scenarios, rolling upgrade/rollback, and performance regression checks. Record the upstream tag and test evidence in the release notes.

Update every field in `upstream/nats-server.lock.json` from the reviewed pull result, then run `make verify-upstream-online`. Never change the lock merely to bless a local source edit; the online tree comparison must remain authoritative.
