"""Regression tests for the signed-commit helper; no GitHub access is needed."""
import base64
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

HELPER = Path(__file__).with_name("commit-signed-changes.py")


class SignedCommitTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        self.env = dict(
            os.environ,
            GITHUB_REPOSITORY="test/repo",
            GIT_CONFIG_GLOBAL=os.devnull,
            GIT_CONFIG_SYSTEM=os.devnull,
        )
        self.git("init", "-q")
        for name in ("modified", "deleted"):
            (self.root / name).write_bytes(b"before\n")
        self.git("add", ".")
        self.git(
            "-c", "user.name=Test", "-c", "user.email=test@example.com",
            "commit", "--no-gpg-sign", "-qm", "initial",
        )
        bindir = self.root / "bin"
        bindir.mkdir()
        gh = bindir / "gh"
        gh.write_text(f"#!{sys.executable}\n" + """
import json
import os
from pathlib import Path
import sys

assert sys.argv[1:] == ["api", "graphql", "--input", "-"]
payload = json.load(sys.stdin)
Path(os.environ["CAPTURE"]).write_text(json.dumps(payload))
mode = os.environ["MODE"]
if mode == "lease":
    print("gh: expectedHeadOid mismatch", file=sys.stderr)
    sys.exit(1)
if mode == "api-error":
    print(json.dumps({"errors": [{"message": "API rejected commit"}]}))
    sys.exit(0)
print(json.dumps({"data": {"createCommitOnBranch": {"commit": {
    "oid": "new-commit",
    "tree": {"oid": "wrong-tree" if mode == "tree" else os.environ["TREE"]},
    "signature": {"isValid": mode != "unsigned", "state": "VALID"}
}}}}))
""")
        gh.chmod(0o755)
        self.env.update(
            PATH=str(bindir) + os.pathsep + os.environ["PATH"],
            CAPTURE=str(self.root / "payload.json"),
        )

    def git(self, *args):
        return subprocess.check_output(
            ["git", "-C", str(self.root), *args], env=self.env,
        ).decode().strip()

    def run_helper(self, mode=""):
        self.env.update(TREE=self.git("write-tree"), MODE=mode)
        return subprocess.run(
            [sys.executable, str(HELPER), "test/branch", 'quoted "message"'],
            cwd=self.root, env=self.env, capture_output=True, text=True,
        )

    def stage_changes(self):
        (self.root / "modified").write_bytes(b"staged\n")
        (self.root / "deleted").unlink()
        (self.root / "new binary").write_bytes(b"\x00\xff\n")
        self.git("add", "-A", "--", "modified", "deleted", "new binary")
        # Serializing the worktree instead of the index must fail the test.
        (self.root / "modified").write_bytes(b"unstaged\n")

    def test_staged_bytes_deletions_and_expected_head(self):
        self.stage_changes()
        result = self.run_helper()
        self.assertEqual(result.returncode, 0, result.stderr)
        payload = json.loads((self.root / "payload.json").read_text())
        request = payload["variables"]["input"]
        self.assertEqual(request["expectedHeadOid"], self.git("rev-parse", "HEAD"))
        self.assertEqual(request["branch"], {
            "repositoryNameWithOwner": "test/repo", "branchName": "test/branch",
        })
        self.assertEqual(request["message"]["headline"], 'quoted "message"')
        self.assertEqual(request["fileChanges"]["deletions"], [{"path": "deleted"}])
        files = {
            entry["path"]: base64.b64decode(entry["contents"])
            for entry in request["fileChanges"]["additions"]
        }
        self.assertEqual(files, {
            "modified": b"staged\n", "new binary": b"\x00\xff\n",
        })
        self.assertIn("Created verified commit new-commit", result.stdout)

    def test_noop_does_not_call_api(self):
        result = self.run_helper()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertFalse((self.root / "payload.json").exists())

    def assert_rejected(self, mode, message):
        self.stage_changes()
        result = self.run_helper(mode)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn(message, result.stderr)
        self.assertNotIn("Created verified commit", result.stdout)

    def test_concurrent_branch_update_is_rejected(self):
        self.assert_rejected("lease", "expectedHeadOid mismatch")

    def test_unsigned_response_is_rejected(self):
        self.assert_rejected("unsigned", "did not verify")

    def test_mismatched_tree_is_rejected(self):
        self.assert_rejected("tree", "does not match")

    def test_graphql_error_is_rejected(self):
        self.assert_rejected("api-error", "API rejected commit")


if __name__ == "__main__":
    unittest.main(verbosity=2)
