import os
from pathlib import Path
import re
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]


class ReleaseToolingTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.directory = Path(self.temporary.name)
        self.bin = self.directory / "bin"
        self.bin.mkdir()
        # No images, services or downloads are touched by installer/gate tests.
        self.executable("docker", """#!/bin/sh
case "$*" in *inspect*) echo false;; esac
exit 0
""")
        self.executable("curl", """#!/bin/sh
while [ "$#" -gt 0 ]; do
 case "$1" in -o) shift; dest="$1";; file://*) source="${1#file://}";; esac
 shift
done
cp "$source" "$dest"
""")
        self.executable("k6", "#!/bin/sh\necho 'unexpected k6 execution' >&2\nexit 99\n")
        self.environment = os.environ.copy()
        for name in list(self.environment):
            if name.startswith(("MAGPIE_", "PERF_", "DB_")) or name in ("JWT_SECRET", "PROXY_ENCRYPTION_KEY"):
                self.environment.pop(name)
        self.environment.update({
            "PATH": str(self.bin) + os.pathsep + os.environ["PATH"],
            "PROXY_ENCRYPTION_KEY": "existing-proxy-key-must-never-change-0123456789",
            "MAGPIE_COMPOSE_URL": (ROOT / "docker-compose.yml").as_uri(),
            "MAGPIE_PRODUCTION_COMPOSE_URL": (ROOT / "docker-compose.production.yml").as_uri(),
            "MAGPIE_ENV_EXAMPLE_URL": (ROOT / ".env.example").as_uri(),
            "PERF_ARTIFACT_DIR": str(self.directory / "artifacts"),
        })

    def executable(self, name, text):
        path = self.bin / name
        path.write_text(text)
        path.chmod(0o755)

    def run_script(self, name, **overrides):
        environment = self.environment | overrides
        return subprocess.run(["bash", str(ROOT / "scripts" / name)], cwd=self.directory,
                              env=environment, capture_output=True, text=True, timeout=15)

    def install(self, name, **overrides):
        directory = self.directory / name
        result = self.run_script("install.sh", MAGPIE_INSTALL_DIR=str(directory), **overrides)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        return directory

    def test_install_randomizes_jwt_and_update_preserves_both_secrets(self):
        first, second = self.install("first"), self.install("second")
        contents = (first / ".env").read_text()
        key = re.search(r'^JWT_SECRET="([0-9a-f]{64})"$', contents, re.M)
        self.assertIsNotNone(key)
        self.assertNotIn(key.group(1), (second / ".env").read_text())
        self.assertIn(self.environment["PROXY_ENCRYPTION_KEY"], contents)
        self.assertEqual((first / ".env").stat().st_mode & 0o777, 0o600)
        self.assertTrue((first / "docker-compose.production.yml").is_file())
        result = self.run_script("update.sh", MAGPIE_INSTALL_DIR=str(first))
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual((first / ".env").read_text(), contents)

    def test_install_keeps_custom_jwt(self):
        directory = self.install("custom", JWT_SECRET="a-custom-jwt-secret-kept-across-upgrades")
        self.assertIn('JWT_SECRET="a-custom-jwt-secret-kept-across-upgrades"', (directory / ".env").read_text())

    def test_release_gate_rejects_missing_soak_or_skipped_drops(self):
        for overrides in [{"PERF_SOAK_DURATION": "0"}, {"PERF_SOAK_DURATION": "2h"}, {"PERF_DROP_BUDGET_SOURCE": "skip"}]:
            result = self.run_script("perf/run-gate.sh", **overrides)
            self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
            self.assertNotIn("unexpected k6 execution", result.stderr)
            self.assertNotIn("PASS", result.stdout)

    def test_release_drop_check_requires_evidence(self):
        for source in ["auto", "skip"]:
            result = self.run_script("perf/check-drop-budget.sh", PERF_DROP_BUDGET_SOURCE=source)
            self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
        result = self.run_script("perf/check-drop-budget.sh", PERF_GATE_MODE="smoke", PERF_DROP_BUDGET_SOURCE="skip")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_drop_budget_fails_for_empty_logs_and_overrun(self):
        log = self.directory / "backend.log"
        for contents, budget, expected in [("", "0", 1), ("INFO backend started\n", "0", 0),
                                           ("WARN dropped_total=5\n", "0", 1), ("WARN dropped_total=5\n", "5", 0)]:
            log.write_text(contents)
            result = self.run_script("perf/check-drop-budget.sh", PERF_DROP_BUDGET_SOURCE="file",
                                     PERF_BACKEND_LOG_FILE=str(log), PERF_PROXY_STAT_DROPS_BUDGET=budget)
            self.assertEqual(result.returncode, expected, result.stdout + result.stderr)


if __name__ == "__main__":
    unittest.main()
