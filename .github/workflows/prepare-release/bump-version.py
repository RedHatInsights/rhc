"""Update the two release versions for prepare-release.yml."""

import os
import re
from pathlib import Path

version = os.environ["RELEASE_VERSION"]
version_pattern = r"[0-9]+\.[0-9]+\.[0-9]+"
if not re.fullmatch(version_pattern, version):
    raise SystemExit("release_version must be X.Y.Z, for example 0.3.14")
if not re.fullmatch(r"(CCT|RHEL)-[1-9][0-9]*", os.environ["JIRA_ISSUE"]):
    raise SystemExit("jira_issue must be a CCT or RHEL card, for example CCT-1234")

updates = {}
versions = []
for name, pattern in (
    ("rhc.spec", r"^Version:[ \t]+([^ \t\r\n]+)[ \t]*\r?$"),
    ("meson.build", r"\bproject\(\s*'rhc',\s*version\s*:\s*'([^']+)'"),
):
    contents = Path(name).read_bytes().decode("utf-8")
    matches = list(re.finditer(pattern, contents, re.MULTILINE))
    if len(matches) != 1 or not re.fullmatch(version_pattern, matches[0].group(1)):
        raise SystemExit(f"Expected one X.Y.Z release version in {name}")
    match = matches[0]
    versions.append(match.group(1))
    updates[name] = contents[: match.start(1)] + version + contents[match.end(1) :]

if versions[0] != versions[1]:
    raise SystemExit("rhc.spec and meson.build must have the same release version")
if tuple(map(int, version.split("."))) <= tuple(map(int, versions[0].split("."))):
    raise SystemExit("release_version must be newer than the current version")

for name, contents in updates.items():
    Path(name).write_bytes(contents.encode("utf-8"))
