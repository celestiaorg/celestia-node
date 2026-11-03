#!/usr/bin/env python3
"""Commit staged changes through GitHub so the commit has a verified signature."""
import base64
import json
import os
import subprocess
import sys


def git(*args):
    return subprocess.check_output(["git", *args])


branch, message = sys.argv[1:]
head = git("rev-parse", "HEAD").decode().strip()
tree = git("write-tree").decode().strip()
entries = git("diff", "--cached", "--name-status", "--no-renames", "-z", "HEAD").split(b"\0")
additions, deletions = [], []
for index in range(0, len(entries) - 1, 2):
    status, path = entries[index].decode(), entries[index + 1].decode()
    if status == "D":
        deletions.append({"path": path})
    elif status in ("A", "M"):
        additions.append({"path": path, "contents": base64.b64encode(
            git("show", ":" + path)).decode()})
    else:
        sys.exit("Unsupported staged change: " + status + " " + path)

if not additions and not deletions:
    print("No staged changes; nothing to commit.")
    sys.exit(0)

payload = {
    "query": """mutation($input: CreateCommitOnBranchInput!) {
      createCommitOnBranch(input: $input) {
        commit { oid tree { oid } signature { isValid state } }
      }
    }""",
    "variables": {"input": {
        "branch": {"repositoryNameWithOwner": os.environ["GITHUB_REPOSITORY"],
                   "branchName": branch},
        "expectedHeadOid": head,
        "message": {"headline": message},
        "fileChanges": {"additions": additions, "deletions": deletions},
    }},
}
# expectedHeadOid rejects concurrent branch updates instead of overwriting them.
response = subprocess.check_output(
    ["gh", "api", "graphql", "--input", "-"],
    input=json.dumps(payload).encode())
result = json.loads(response)
if result.get("errors"):
    sys.exit(json.dumps(result["errors"]))
commit = result["data"]["createCommitOnBranch"]["commit"]
if commit["tree"]["oid"] != tree:
    sys.exit("Remote commit tree does not match the staged changes.")
if not (commit.get("signature") or {}).get("isValid"):
    sys.exit("GitHub did not verify the new commit signature.")
print("Created verified commit " + commit["oid"])
