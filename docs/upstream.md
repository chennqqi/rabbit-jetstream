# Upstream NATS Server

`upstream/nats-server/` is a Git subtree pinned to the official `nats-io/nats-server` repository. The initial pin is `v2.14.1`.

The subtree is the message-server core of this distribution. Management APIs, UI, packaging, deployment defaults, and operational extensions belong outside this directory. Direct upstream edits require an architecture decision record explaining why configuration or an external component cannot solve the problem, plus a plan to upstream or continuously rebase the patch.

## Updating

Start from a clean branch, review the upstream release and upgrade notes, then run:

```bash
git subtree pull \
  --prefix=upstream/nats-server \
  https://github.com/nats-io/nats-server.git \
  <stable-tag> --squash
```

After an update, build the upstream server and run its tests, this repository's test suites, three-node recovery scenarios, rolling upgrade/rollback, and performance regression checks. Record the upstream tag and test evidence in the release notes.
