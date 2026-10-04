---
name: rjs-release-drills
description: Run and evaluate Rabbit JetStream release qualification drills - local-rc packaging, bare-metal soak, helm/Kind cluster smoke, five-stage canary, node-failure and rollback drills. Use when preparing a release candidate, re-running qualification gates after merges, or assembling/refreshing release-approval evidence.
---

# Rabbit JetStream release drills

## Gate order (all must pass on the SAME frozen tree)

1. **Local gates**: `go test ./...`, `go vet ./...`, admin-ui `npm test && npm run build && npm run promote && npm run verify:dist`, contract tests (`go test ./tests/contract/`).
2. **Packaging**: `pwsh -NoProfile -File scripts/release/local-rc.ps1 -Mode Quick -Output artifacts/local-rc.json` (Release mode adds race/coverage/backup/rolling/migration/helm/security/performance gates; requires the SDK repo at outlink/rabbit-jetstream-go). Evidence: `artifacts/local-rc.json`.
3. **Bare-metal soak** (24 h minimum, host `ssh jdcloudremote` as sandbox): script `tests/soak/rc3-bare-metal.sh`. Acceptance: evidence cycles all health=200, queues_api=200, zero restarts, management log zero errors, messages sustained. Evidence: `/home/sandbox/rc3/logs/soak-evidence.log`.
4. **Cluster smoke** (helm/Kind 3 workers): `tests/deployment/kubernetes-smoke.sh` semantics; podman adaptations in `tests/soak/rc3-cluster-install.sh` + `rc3-cluster-verify.sh` + `run-rc3-cluster-smoke.sh`. Acceptance: rollouts, 3-node spread, PVCs Bound, auth secret stable, NetworkPolicy positive+negative, PDB second evictions denied, readyz+admin UI+cluster profile, clean uninstall with PVCs retained.
5. **Canary five stages** (1/10/25/50/100 %): `tests/soak/rc3-canary-drill.sh` on the 3-node host. Acceptance per stage: `received == published`, corrupt=0, duplicate=0, management_5xx=0, JetStream+controller+3 nodes true. NOTE: reconcile uses delivery accounting (workqueue deletes on ack — do not use ack floors or seq counts).
6. **Node-failure drill**: kill one nats node at the 10 % share mid-run; acceptance: publish errors < 20 % of published (client retry absorbs the quorum gap), received >= 90 % of published after redelivery, replicas converge. `tests/soak/rc3-node-rollback-drill.sh`.
7. **Rollback drill**: restart the management service mid-traffic; acceptance: readyz back within budget, stream reconciled. Same script.

## Rules

- Qualification binds to an exact tree: re-run gates after ANY merge into the release branch. Record the revision (`git rev-parse HEAD`) in every evidence file.
- Never copy the source tree to the qualification host; transfer frozen binaries + checksums only (`GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ...`, sha256sum, verify after transfer).
- Registry access is not assumed: base images via mirrors (gcr.m.daocloud.io / docker.m.daocloud.io), candidate images loaded from per-image archives (multi-image archives collapse shared-base images).
- Evidence format follows docs/releases/v0.1.0-rc.3-*.md and the release-approval template (`docs/release-approval.template.json`): canary stages, node_failure, rollback, partial_dependencies, signoffs.
- Frozen-candidate etiquette: qualification binds to an exact tree — re-run gates after any merge into the release branch; record the revision in every evidence file; tag only after all gates pass (tag message lists the evidence set).
