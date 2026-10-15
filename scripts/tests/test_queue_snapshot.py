import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

from redis_fixture import RedisFixture


ROOT = Path(__file__).resolve().parents[2]
REDIS_URL = os.environ.get("MAGPIE_TEST_REDIS_URL")


@unittest.skipUnless(REDIS_URL, "MAGPIE_TEST_REDIS_URL must point to a disposable Redis service")
class QueueSnapshotTests(unittest.TestCase):
    def setUp(self):
        # This database belongs exclusively to this integration fixture.
        self.redis = RedisFixture(REDIS_URL)
        self.addCleanup(self.redis.close)
        self.redis.command("FLUSHDB")
        self.addCleanup(self.redis.command, "FLUSHDB")
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.directory = Path(self.temporary.name)
        docker = self.directory / "docker"
        docker.write_text("#!" + sys.executable + "\n" + '''
import os, sys
from pathlib import Path
sys.path.insert(0, os.environ["MOCK_TEST_MODULES"])
from redis_fixture import RedisFixture
args = sys.argv[1:]
if "inspect" in args:
    print("true")
elif "ps" in args:
    print(args[-1])
elif "exec" in args and "redis" in args:
    client = RedisFixture(os.environ["MAGPIE_TEST_REDIS_URL"])
    script = args[-1]
    if os.environ.get("MAGPIE_TEST_CAPTURE_LUA"):
        Path(os.environ["MAGPIE_TEST_CAPTURE_LUA"]).write_text(script)
    for count in client.command("EVAL", script, 0):
        print(count)
    client.close()
elif "exec" in args and "postgres" in args:
    print("11|22|0|12345|67890")
elif args != ["compose", "version"]:
    sys.exit("Unexpected Docker call: " + repr(args))
''')
        docker.chmod(0o755)

    def capture(self):
        before = self.redis.command_calls()
        environment = os.environ | {
            "PATH": str(self.directory) + os.pathsep + os.environ["PATH"],
            "MOCK_TEST_MODULES": str(ROOT / "scripts/tests"),
        }
        result = subprocess.run(["bash", str(ROOT / "scripts/perf/capture-snapshot.sh")],
                                env=environment, capture_output=True, text=True, timeout=20)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        after = self.redis.command_calls()
        self.assertEqual(after.get("zcard", 0) - before.get("zcard", 0), 194)
        self.assertEqual(after.get("scan", 0), before.get("scan", 0))
        self.assertEqual(after.get("keys", 0), before.get("keys", 0))
        values = dict(line.split("=", 1) for line in result.stdout.splitlines())
        self.assertEqual(values["proxy_queue_depth"], "10")
        self.assertEqual(values["scrapesite_queue_depth"], "12")
        self.assertEqual(values["proxy_statistics_total_bytes"], "12345")

    def test_legacy_current_and_highest_shards_without_keyspace_traversal(self):
        for key, count in {"proxy_queue": 2, "proxy_queue:0": 3, "proxy_queue:127": 5,
                           "scrapesite_queue": 1, "scrapesite_queue:0": 4,
                           "scrapesite_queue:63": 7}.items():
            self.redis.command("ZADD", key, *[part for i in range(count) for part in (i, str(i))])
        self.redis.command("SET", "proxy_queue:metadata", "not a sorted set")
        self.redis.command("SET", "scrapesite_queue:metadata", "not a sorted set")
        self.capture()
        for start in range(0, 10000, 1000):
            self.redis.pipeline(("SET", "proxy:" + str(i), "payload") for i in range(start, start + 1000))
        self.capture()


if __name__ == "__main__":
    unittest.main()
