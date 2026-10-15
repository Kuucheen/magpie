import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
DOCKER = shutil.which("docker")
PWSH = os.environ.get("MAGPIE_TEST_PWSH") or shutil.which("pwsh")
DOCKER_INTEGRATION = DOCKER and os.environ.get("MAGPIE_TEST_DOCKER_INTEGRATION") == "1"


class BashUpdateTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.directory = Path(self.temporary.name)
        self.install = self.directory / "install with spaces"
        self.install.mkdir()
        self.bin = self.directory / "bin"
        self.bin.mkdir()
        self.calls = self.directory / "docker.jsonl"
        self.project = "magpie-existing"
        self.files = [self.install / "docker-compose.yml",
                      self.install / "docker-compose.production.yml"]
        for file in self.files:
            shutil.copyfile(ROOT / file.name, file)
        self.dotenv = 'PROXY_ENCRYPTION_KEY=existing-stable-proxy-key-0123456789012345\nJWT_SECRET=existing-custom-jwt-secret-01234567890123456789\n'
        (self.install / ".env").write_text(self.dotenv)
        self.environment = {k: v for k, v in os.environ.items()
                            if not k.startswith(("MAGPIE_", "COMPOSE_", "MOCK_"))}
        for key in ("MAGPIE_ENV", "JWT_SECRET", "PROXY_ENCRYPTION_KEY"):
            self.environment.pop(key, None)
        self.environment.update({
            "PATH": str(self.bin) + os.pathsep + os.environ["PATH"],
            "MAGPIE_INSTALL_DIR": str(self.install),
            "MAGPIE_COMPOSE_URL": (ROOT / "docker-compose.yml").as_uri(),
            "MAGPIE_PRODUCTION_COMPOSE_URL": (ROOT / "docker-compose.production.yml").as_uri(),
            "MAGPIE_ENV_EXAMPLE_URL": (ROOT / ".env.example").as_uri(),
            "MOCK_CALLS": str(self.calls),
            "MOCK_FILES": json.dumps([str(f) for f in self.files]),
            "MOCK_PROJECT": self.project,
            "MOCK_DIRECTORY": str(self.install),
        })
        self.executable("docker", '''
import json, os, sys
args = sys.argv[1:]
with open(os.environ["MOCK_CALLS"], "a") as f:
    f.write(json.dumps(args) + "\\n")
operation = next((a for a in args if a in ("ps", "inspect", "config", "pull", "stop", "run", "up")), "")
if os.environ.get("MOCK_FAILURE") == operation and operation:
    sys.exit(1)
labels = {
    "com.docker.compose.project.config_files": ",".join(json.loads(os.environ["MOCK_FILES"])),
    "com.docker.compose.project": os.environ["MOCK_PROJECT"],
    "com.docker.compose.project.working_dir": os.environ["MOCK_DIRECTORY"],
    "com.docker.compose.oneoff": "False",
}
containers = json.loads(os.environ["MOCK_CONTAINERS"]) if "MOCK_CONTAINERS" in os.environ else {"existing-backend-id": labels}
if operation == "ps" and not os.environ.get("MOCK_NO_BACKEND"):
    # A lookup inferred from the base file misses an overlay's project name.
    if not (args[0] == "compose" and os.environ.get("MOCK_NAMED_PROJECT")):
        print("\\n".join(containers))
elif operation == "inspect":
    labels = containers[args[-1]]
    if "{{json .Config.Labels}}" in args:
        print(json.dumps(labels))
    else:
        print("\\n".join(labels.get(key, "") for key in (
            "com.docker.compose.project.config_files", "com.docker.compose.project",
            "com.docker.compose.project.working_dir", "com.docker.compose.oneoff")))
elif operation in ("pull", "stop", "run", "up"):
    print("operation succeeded")
''')
        self.executable("curl", '''
import shutil, sys
from urllib.parse import urlparse, unquote
args = sys.argv[1:]
url = next(a for a in args if a.startswith("file://"))
shutil.copyfile(unquote(urlparse(url).path), args[args.index("-o") + 1])
''')

    def executable(self, name, contents):
        path = self.bin / name
        path.write_text("#!" + sys.executable + "\n" + contents)
        path.chmod(0o755)

    def run_update(self, local=False, script=None, **overrides):
        return subprocess.run(["bash", str(script or ROOT / "scripts/update.sh")]
                              + (["--local-config"] if local else []),
                              cwd=self.directory, env=self.environment | overrides,
                              capture_output=True, text=True, timeout=20)

    def mutations(self):
        calls = [json.loads(line) for line in self.calls.read_text().splitlines()]
        return [call for call in calls if any(action in call for action in ("pull", "stop", "run", "up"))]

    def config_flags(self, files):
        identity = ["-p", self.project, "--project-directory", str(self.install)] if files else []
        return identity + [item for file in files for item in ("-f", str(file))]

    def assert_update(self, result, files):
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        expected_flags = self.config_flags(files)
        actions = [["pull"], ["stop", "backend"],
                   ["run", "--rm", "backend", "--migrate-only"], ["up", "-d"]]
        actual = self.mutations()
        self.assertEqual(len(actual), len(actions), actual)
        for command, action in zip(actual, actions):
            self.assertEqual(command[:1 + len(expected_flags) + len(action)],
                             ["compose"] + expected_flags + action)
        self.assertEqual((self.install / ".env").read_text(), self.dotenv)

    def assert_policy(self, files, production, **overrides):
        if not DOCKER:
            return
        result = subprocess.run([DOCKER, "compose"]
                                + self.config_flags(files)
                                + ["config", "--format", "json"], cwd=self.install,
                                env=self.environment | overrides, text=True,
                                capture_output=True, timeout=20)
        self.assertEqual(result.returncode, 0, result.stderr)
        configuration = json.loads(result.stdout)
        if files:
            self.assertEqual(configuration["name"], self.project)
        backend = configuration["services"]["backend"]
        self.assertEqual(backend["environment"]["MAGPIE_ENV"], "production" if production else "development")
        self.assertEqual(backend["environment"]["ALLOW_INSECURE_REGISTRATION_DEFAULTS"], "false" if production else "true")
        if production:
            self.assertEqual(backend["environment"]["DISABLE_PUBLIC_REGISTRATION"], "true")
            self.assertEqual(backend["environment"]["STRICT_SECRET_VALIDATION"], "true")
            self.assertIn("-production", backend["command"])

    def test_overlay_only_production_and_custom_file_order_survive_update(self):
        custom = self.install / "custom settings.yml"
        custom.write_text('services:\n  backend:\n    environment:\n      LOG_LEVEL: warn\n')
        files = self.files + [custom]
        result = self.run_update(MOCK_FILES=json.dumps([str(f) for f in files]))
        self.assert_update(result, files)
        self.assert_policy(files, production=True)
        calls = [json.loads(line) for line in self.calls.read_text().splitlines()]
        self.assertIn(["ps", "-a", "-q", "--filter", "label=com.docker.compose.service=backend"], calls)

    def test_named_project_survives_overlay_refresh(self):
        self.project = "magpie-production"
        self.files[1].write_text("name: magpie-production\n" + self.files[1].read_text())
        result = self.run_update(MOCK_PROJECT=self.project, MOCK_NAMED_PROJECT="1")
        self.assert_update(result, self.files)
        # Downloaded configuration no longer declares the original project name.
        self.assertNotIn("name: magpie-production", self.files[1].read_text())
        self.assert_policy(self.files, production=True)

    def candidate_labels(self, project=None, directory=None, files=None, oneoff="False"):
        return {
            "com.docker.compose.project": project or self.project,
            "com.docker.compose.project.config_files": ",".join(str(f) for f in (files or self.files)),
            "com.docker.compose.project.working_dir": str(directory or self.install),
            "com.docker.compose.oneoff": oneoff,
        }

    def test_unrelated_projects_and_one_off_containers_are_ignored(self):
        other = self.directory / "other installation"
        other.mkdir()
        containers = {
            "unrelated": self.candidate_labels(project="unrelated", directory=other,
                                               files=[other / "docker-compose.yml"]),
            "migration": self.candidate_labels(project="old-migration", oneoff="True"),
            "selected": self.candidate_labels(),
        }
        self.assert_update(self.run_update(MOCK_CONTAINERS=json.dumps(containers)), self.files)

    def test_ambiguous_projects_abort_before_mutations(self):
        containers = {"one": self.candidate_labels(),
                      "two": self.candidate_labels(project="another-project")}
        result = self.run_update(MOCK_CONTAINERS=json.dumps(containers))
        self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(self.mutations(), [])

    @unittest.skipUnless(DOCKER_INTEGRATION, "MAGPIE_TEST_DOCKER_INTEGRATION=1 enables disposable Docker fixtures")
    def test_real_docker_discovers_project_named_only_in_overlay(self):
        # Create only, never start. All update mutations below are intercepted.
        self.project = "magpie-update-" + self.directory.name.lower()
        self.files[0].write_text('services:\n  backend:\n    image: alpine:3.23\n    command: [sleep, "3600"]\n')
        self.files[1].write_text("name: " + self.project + "\n" + self.files[1].read_text())
        args = [DOCKER, "compose"] + [item for file in self.files for item in ("-f", str(file))]
        self.assertNotIn("COMPOSE_PROJECT_NAME", self.environment)
        created = subprocess.run(args + ["create", "backend"], cwd=self.install,
                                 env=self.environment, capture_output=True, text=True, timeout=30)
        self.assertEqual(created.returncode, 0, created.stdout + created.stderr)
        self.addCleanup(subprocess.run, [DOCKER, "network", "rm", self.project + "_default"], capture_output=True)
        cid = subprocess.run(args + ["ps", "-a", "-q", "backend"], cwd=self.install,
                             env=self.environment, capture_output=True, text=True, timeout=15)
        self.assertEqual(cid.returncode, 0, cid.stderr)
        self.addCleanup(subprocess.run, [DOCKER, "rm", "-f", cid.stdout.strip()], capture_output=True)
        self.environment["MOCK_REAL_DOCKER"] = DOCKER
        self.executable("docker", '''
import json, os, subprocess, sys
args = sys.argv[1:]
with open(os.environ["MOCK_CALLS"], "a") as f:
    f.write(json.dumps(args) + "\\n")
if any(action in args for action in ("pull", "stop", "run", "up")):
    print("intercepted update mutation")
else:
    sys.exit(subprocess.run([os.environ["MOCK_REAL_DOCKER"], *args]).returncode)
''')
        self.assert_update(self.run_update(), self.files)
        self.assertNotIn("name: " + self.project, self.files[1].read_text())
        self.assert_policy(self.files, production=True)

    def test_local_update_does_not_enable_production_overlay(self):
        files = self.files[:1]
        self.assert_update(self.run_update(MOCK_FILES=json.dumps([str(f) for f in files])), files)
        self.assert_policy(files, production=False)

    def test_missing_or_unreadable_selection_aborts_before_mutations(self):
        for overrides in [{"MOCK_FILES": "[]"}, {"MOCK_FILES": '["/missing/production.yml"]'},
                          {"MOCK_PROJECT": ""},
                          {"MOCK_FAILURE": "inspect"}, {"MOCK_FAILURE": "ps"},
                          {"MOCK_FAILURE": "config"}]:
            with self.subTest(overrides=overrides):
                self.calls.unlink(missing_ok=True)
                result = self.run_update(**overrides)
                self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertEqual(self.mutations(), [])
                self.assertEqual((self.install / ".env").read_text(), self.dotenv)

    def test_without_container_compose_file_setting_is_honored(self):
        selection = os.pathsep.join(str(f) for f in self.files)
        for dotenv in (False, True):
            with self.subTest(dotenv=dotenv):
                self.calls.unlink(missing_ok=True)
                if dotenv:
                    self.dotenv += "COMPOSE_FILE=" + selection + "\n"
                    (self.install / ".env").write_text(self.dotenv)
                overrides = {} if dotenv else {"COMPOSE_FILE": selection}
                self.assert_update(self.run_update(MOCK_NO_BACKEND="1", **overrides), [])
                self.assert_policy([], production=True, **overrides)

    def test_local_configuration_path_preserves_overlay_and_custom_files(self):
        # Used by the checkout updaters after git pull, with no configuration downloads.
        (self.install / "docker-compose.yml.bak").unlink(missing_ok=True)
        self.assert_update(self.run_update(local=True, MAGPIE_COMPOSE_URL="file:///must-not-download"), self.files)
        self.assertFalse((self.install / "docker-compose.yml.bak").exists())

    def test_failed_migration_leaves_backend_stopped_and_retry_keeps_selection(self):
        failed = self.run_update(MOCK_FAILURE="run")
        self.assertNotEqual(failed.returncode, 0, failed.stdout + failed.stderr)
        expected_flags = self.config_flags(self.files)
        mutations = self.mutations()
        self.assertEqual(len(mutations), 3, mutations)
        self.assertEqual(mutations[-1], ["compose"] + expected_flags + ["run", "--rm", "backend", "--migrate-only"])
        self.calls.unlink()
        self.assert_update(self.run_update(), self.files)

    def test_git_checkout_helper_uses_same_selection(self):
        scripts = self.install / "scripts"
        scripts.mkdir()
        for name in ("update-stack.sh", "update.sh"):
            shutil.copyfile(ROOT / "scripts" / name, scripts / name)
        self.executable("git", "import sys\nsys.exit(0)\n")
        self.assert_update(self.run_update(script=scripts / "update-stack.sh"), self.files)


@unittest.skipUnless(PWSH, "pwsh is required to execute PowerShell updater regressions")
class PowerShellUpdateTests(BashUpdateTests):
    def run_update(self, local=False, script=None, **overrides):
        if script is not None:
            self.skipTest("the Windows CMD checkout wrapper requires Windows")
        driver = self.directory / "driver.ps1"
        driver.write_text('''
function Invoke-WebRequest {
  param([uri]$Uri, [string]$OutFile, [switch]$UseBasicParsing)
  Copy-Item -LiteralPath $Uri.LocalPath -Destination $OutFile -Force
}
& $env:MAGPIE_TEST_SCRIPT ''' + ("-LocalConfig" if local else "") + "\n")
        return subprocess.run([PWSH, "-NoLogo", "-NoProfile", "-File", str(driver)],
                              cwd=self.directory,
                              env=self.environment | {"MAGPIE_TEST_SCRIPT": str(ROOT / "scripts/update.ps1")} | overrides,
                              capture_output=True, text=True, timeout=20)


if __name__ == "__main__":
    unittest.main()
