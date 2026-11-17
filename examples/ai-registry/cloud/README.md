English | [Chinese](README.zh_CN.md)

# Remote AI Registry E2E

`cloud/` builds a self-contained package for running the AI Registry real E2E
suite on another host. The package contains prebuilt Provider, Consumer, E2E,
and cleanup binaries, the existing `verify_all.sh` workflow, result
summarization, log transfer, and ownership-scoped cleanup helpers.

The target host does not need Go or access to the project source tree.

## Build

From the repository root:

```bash
./examples/ai-registry/cloud/build-package.sh
```

Default output:

```text
examples/ai-registry/cloud/dist/go-agent-naming-polarismesh-cloud-<version>-<commit>-<time>.tar.gz
examples/ai-registry/cloud/dist/go-agent-naming-polarismesh-cloud-<version>-<commit>-<time>.tar.gz.sha256
```

The default target is Linux/amd64. Build for another architecture with:

```bash
./examples/ai-registry/cloud/build-package.sh \
  --goos linux \
  --goarch arm64
```

Use `--name` and `--output-dir` for custom names or locations. `--clean`
removes local cloud build and output directories.

The package uses `CGO_ENABLED=0` and trimmed binaries. Its manifest records the
adapter version, Git commit, dirty state, build time, target platform, and Go
version.

## Upload

```bash
scp examples/ai-registry/cloud/dist/<package>.tar.gz \
    examples/ai-registry/cloud/dist/<package>.tar.gz.sha256 \
    <user>@<cloud-host>:~/
```

On the remote host:

```bash
sha256sum -c <package>.tar.gz.sha256
tar xzf <package>.tar.gz
cd <package>
```

Use `shasum -a 256 -c` where GNU `sha256sum` is unavailable.

## Run

The remote host needs `bash`, `tar`, `jq`, `ps`, `curl`, and the usual process
inspection tools. The prebuilt package does not require Go.

Retained resources:

```bash
POLARIS_TOKEN=<optional> \
POLARIS_SERVICE_TOKEN=<optional> \
./run-e2e.sh \
  --server <polaris-host> \
  --resources keep
```

Automatic ownership-scoped cleanup:

```bash
./run-e2e.sh \
  --server <polaris-host> \
  --resources cleanup \
  --run-id review-run
```

Custom ports and addresses:

```bash
./run-e2e.sh \
  --server <polaris-host> \
  --discover <polaris-host>:8091 \
  --console http://<polaris-host>:8090 \
  --skill-address <polaris-host>:8094 \
  --namespace go-agent-naming-e2e \
  --timeout 10s \
  --run-timeout 20m
```

The default `keep` policy retains state-owned resources and the final Provider.
Use `cleanup` when the run should remove its instances, services, and Skills.
Cleanup uses the existing owner, run ID, PID, process-start-time, command, and
instance checks.

Credentials are read only from:

- `POLARIS_TOKEN`
- `POLARIS_CONSOLE_TOKEN`
- `POLARIS_SKILL_TOKEN`
- `POLARIS_SERVICE_TOKEN`

Tokens are not written to `environment.txt`, manifests, state, or summaries.

## Run Directory

```text
cloud-runs/<run-id>/
├── analysis.md
├── environment.txt
├── manifest.json
├── results.json
├── run.log
├── state.json
└── providers/
```

Additional service and Skill projection diagnostics may appear when executed
by the suite.

## Logs

`run-e2e.sh` creates a checksummed archive by default:

```text
cloud-runs/<run-id>-logs.tar.gz
cloud-runs/<run-id>-logs.tar.gz.sha256
```

Manual packing:

```bash
./pack-logs.sh --run-dir cloud-runs/<run-id>
./pack-logs.sh --all
```

Transfer and import locally:

```bash
scp <user>@<cloud-host>:~/cloud-runs/<run-id>-logs.tar.gz .
scp <user>@<cloud-host>:~/cloud-runs/<run-id>-logs.tar.gz.sha256 .

./examples/ai-registry/cloud/import-logs.sh ./<run-id>-logs.tar.gz
```

The importer verifies the adjacent checksum, rejects absolute paths, parent
traversal, and archive links, then generates `analysis.md`.

## Cleanup

Preview:

```bash
./cleanup-run.sh --run-id <run-id> --dry-run
```

Apply:

```bash
./cleanup-run.sh \
  --run-id <run-id> \
  --apply \
  --delete-service
```

Cleanup is limited to the run state and resources selected by the existing
ownership rules.

## Security

- Do not put tokens in URLs or command-line arguments.
- Do not store access-bearing URLs in state or results.
- Treat remote logs as internal data because they may contain service names,
  instance IDs, interface URLs, and request diagnostics.
- Do not bypass ownership and process identity checks with broad name-based
  deletion.
