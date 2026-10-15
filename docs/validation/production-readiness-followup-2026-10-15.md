# Release tooling review fixes, 15 October 2026

Both findings from the follow-up review are fixed in the deployment repository.

## Updates preserve production configuration

The Bash and PowerShell updaters discover backend containers directly through
Docker's service labels, independently of the project inferred from the base
file. They match this installation's recorded working directory or base file
and recover the project name, working directory and full ordered Compose file
list. Unrelated projects and one-off migration containers are ignored.

Every configuration check, pull, stop, migration and recreation passes the
recorded project explicitly with `-p` and the original `--project-directory`
and `-f` arguments. This preserves names selected by an overlay's `name:`, `-p`
or `COMPOSE_PROJECT_NAME`, even when downloaded files change or remove the
original name. The existing project's volumes and networks stay selected.
Stopped containers are included, so a failed migration can be retried with
the same configuration. Missing files, ambiguous matching projects and
inspection errors abort before backend mutations. Secrets remain unchanged.

The Git checkout helpers call the same update logic with local configuration,
after pulling and restoring checkout changes. The PowerShell native-command
helper also keeps stdout separate from the exit code, so successful commands
that print output are accepted.

When containers have been removed with `docker compose down`, the updater uses
Compose's normal settings. Persist `COMPOSE_FILE` and any custom
`COMPOSE_PROJECT_NAME` in the shell or `.env` to retain the selection without
container metadata. The README, environment example and deployment/update
documentation describe this case and the Windows path separator.

## Queue snapshots use bounded Redis work

The snapshot counts the legacy proxy/scrape queues, proxy shards `0` through
`127`, and scrape shards `0` through `63`. These are the backend's supported
limits. A short Lua aggregate issues exactly 194 constant-time `ZCARD` reads.
It includes earlier shard layouts and performs no `SCAN` or `KEYS` calls.
Keep these bounds aligned with the backend when changing its shard limits.

The following comparison used a disposable Redis 7.4.8 container with
1,000,000 `proxy:*` payload keys and one member in `proxy_queue:127`. A second
connection issued `PING` while each script ran. The old script ran five times;
the fixed script ran 100 times. Both returned queue totals `[1, 0]`.

| Measurement | Previous full scan | Fixed bounded aggregate |
| --- | ---: | ---: |
| Median snapshot duration | 345.934 ms | 0.385 ms |
| Maximum snapshot duration | 377.023 ms | 0.788 ms |
| Longest concurrent ping | 376.372 ms | 0.530 ms |
| SCAN calls across the run | 5,000 | 0 |
| ZCARD calls per snapshot | 3 | 194 |

These are local latency measurements, not a checker throughput or production
capacity result. Raw measurements are in
[`snapshot-latency-2026-10-15.json`](./snapshot-latency-2026-10-15.json).

## Verification

- Twenty-seven release tooling tests passed with Bash, PowerShell 7.5.0 on
  Linux and a disposable Redis service. The Windows CMD wrapper test was
  skipped. The [complete test log](./updater-project-tests-2026-10-15.log)
  records every Bash and PowerShell result. They cover overlay-only production, local mode,
  custom overlay ordering, unchanged secrets, missing/unreadable selection,
  migration failure/retry, local checkout configuration and `COMPOSE_FILE`
  supplied through the shell or `.env`.
- Both update helpers passed a real Docker regression that creates a stopped
  backend with its project named only by the production overlay. No
  `COMPOSE_PROJECT_NAME` is set. The test refreshes the overlay, removing its
  original name, and confirms that all update commands keep the original
  project and file order. Actual Compose resolution retains `-production`,
  strict secret validation and disabled public registration. Docker discovery,
  inspection and configuration validation are real; pull/stop/migrate/up
  mutations are intercepted. Fixtures are removed after the test.
- The snapshot regression executes the script's actual Lua against Redis,
  checks legacy and highest supported shards, ignores metadata keys, and proves
  exactly 194 cardinality reads with zero scans before and after adding 10,000
  payload keys.
- Bash syntax checks, production Compose configuration validation, whitespace
  checks and the Docusaurus production build passed.
- CI supplies disposable Redis, enables the Docker regressions and requires a
  PowerShell runtime before running the suite.

Run the tooling suite with a disposable Redis database:

```bash
MAGPIE_TEST_REDIS_URL=redis://127.0.0.1:6379/15 \
MAGPIE_TEST_DOCKER_INTEGRATION=1 \
python3 -m unittest discover -s scripts/tests -v
```

The Docker fixture needs `alpine:3.23` locally, as provided by CI's image pull.
The Redis fixture clears its selected database. `pwsh` must be on `PATH`, or
`MAGPIE_TEST_PWSH` can point to a portable runtime. Set
`MAGPIE_TEST_CAPTURE_LUA=/tmp/queue-snapshot.lua` to save the Lua received by
the Redis fixture for separate latency measurements.

No Go source changed in this follow-up. The previous normal/race Go, browser,
container and shutdown validation still applies to those unchanged sources.
Native Windows PowerShell/CMD execution, a representative soak and a backup
restore exercise remain outstanding.
