#!/usr/bin/env python3
"""Select Go and integration tests from changed files.

This script is intentionally deterministic and easy to reason about.
It is designed for a "shadow mode" rollout where selection quality is measured
before it is used for CI gating.
"""

from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
from pathlib import Path
from typing import Any

try:
    import yaml
except ImportError as exc:  # pragma: no cover
    raise SystemExit(
        "PyYAML is required. Install with: pip install pyyaml"
    ) from exc


def compile_patterns(patterns: list[str]) -> list[re.Pattern[str]]:
    return [re.compile(pattern) for pattern in patterns]


def matches_any(path: str, patterns: list[re.Pattern[str]]) -> bool:
    return any(pattern.search(path) for pattern in patterns)


def git_changed_files(base: str, head: str) -> list[str]:
    diff_ref = f"{base}...{head}"
    result = subprocess.run(
        ["git", "diff", "--name-only", diff_ref],
        check=True,
        capture_output=True,
        text=True,
    )
    files = [line.strip() for line in result.stdout.splitlines() if line.strip()]
    return sorted(set(files))


def normalize_go_package(path: str) -> str | None:
    if not path.endswith(".go"):
        return None
    parts = Path(path).parts
    if not parts:
        return None
    if parts[0] not in ("cmd", "internal", "pkg"):
        return None
    parent = str(Path(path).parent)
    return f"./{parent}"


def write_github_outputs(path: str, selection: dict[str, Any]) -> None:
    with open(path, "a", encoding="utf-8") as file:
        file.write(
            f"run_full_integration={'true' if selection['run_full_integration'] else 'false'}\n"
        )
        file.write(f"risk_level={selection['risk_level']}\n")
        file.write(f"go_packages={','.join(selection['selected_go_packages'])}\n")
        file.write(f"pytest_files={','.join(selection['selected_pytest_files'])}\n")
        file.write(f"changed_files_count={len(selection['changed_files'])}\n")
        file.write(f"docs_only={'true' if selection['docs_only_change'] else 'false'}\n")


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Select test targets based on changed files."
    )
    parser.add_argument("--base", required=True, help="Base git ref/sha")
    parser.add_argument("--head", required=True, help="Head git ref/sha")
    parser.add_argument(
        "--map-file",
        default="scripts/its_test_map.yaml",
        help="Path to YAML map describing test selection rules.",
    )
    parser.add_argument(
        "--output-json",
        default="selector-output.json",
        help="Where to write structured selection output.",
    )
    parser.add_argument(
        "--github-output",
        default="",
        help="Optional path to GitHub output file ($GITHUB_OUTPUT).",
    )
    args = parser.parse_args()

    with open(args.map_file, "r", encoding="utf-8") as file:
        config = yaml.safe_load(file)

    changed_files = git_changed_files(args.base, args.head)
    ignore_patterns = compile_patterns(config.get("ignore_patterns", []))
    docs_patterns = compile_patterns(config.get("docs_only_patterns", []))
    fallback_patterns = compile_patterns(config.get("fallback_patterns", []))

    considered_files = [
        path for path in changed_files if not matches_any(path, ignore_patterns)
    ]
    docs_only_change = bool(considered_files) and all(
        matches_any(path, docs_patterns) for path in considered_files
    )

    selected_go_packages: set[str] = set()
    selected_pytest_files: set[str] = set()
    matched_rules: list[str] = []
    reasons: list[str] = []

    smoke = config.get("smoke", {})
    smoke_go = set(smoke.get("go_packages", []))
    smoke_pytest = set(smoke.get("pytest_files", []))

    rule_match_by_file: dict[str, list[str]] = {path: [] for path in considered_files}
    for rule in config.get("rules", []):
        compiled_rule_patterns = compile_patterns(rule.get("patterns", []))
        matched = [
            path for path in considered_files if matches_any(path, compiled_rule_patterns)
        ]
        if not matched:
            continue
        matched_rules.append(rule["id"])
        selected_go_packages.update(rule.get("go_packages", []))
        selected_pytest_files.update(rule.get("pytest_files", []))
        reasons.append(f"rule:{rule['id']}")
        for path in matched:
            rule_match_by_file[path].append(rule["id"])

    for path in considered_files:
        if path.startswith("integration-tests/test_") and path.endswith(".py"):
            selected_pytest_files.add(path)
            reasons.append(f"direct-test-file:{path}")
        package = normalize_go_package(path)
        if package and not rule_match_by_file[path]:
            selected_go_packages.add(package)
            reasons.append(f"derived-go-package:{package}")

    full_run_triggers = [
        path for path in considered_files if matches_any(path, fallback_patterns)
    ]
    run_full_integration = bool(full_run_triggers)

    if not considered_files:
        run_full_integration = True
        reasons.append("no-considered-files")

    if run_full_integration:
        reasons.extend(f"full-trigger:{path}" for path in full_run_triggers)
    else:
        selected_go_packages.update(smoke_go)
        # Keep docs-only changes very cheap while still exercising basic CLI output.
        selected_pytest_files.update(smoke_pytest)
        if docs_only_change:
            reasons.append("docs-only-change")

    if run_full_integration:
        risk_level = "high"
    elif docs_only_change:
        risk_level = "low"
    elif matched_rules:
        risk_level = "medium"
    else:
        risk_level = "medium"

    selection = {
        "base": args.base,
        "head": args.head,
        "changed_files": changed_files,
        "considered_files": considered_files,
        "docs_only_change": docs_only_change,
        "matched_rules": sorted(set(matched_rules)),
        "selected_go_packages": sorted(selected_go_packages),
        "selected_pytest_files": sorted(selected_pytest_files),
        "run_full_integration": run_full_integration,
        "risk_level": risk_level,
        "reasons": sorted(set(reasons)),
        "recommended_commands": {
            "go": (
                f"go test {' '.join(sorted(selected_go_packages))}"
                if selected_go_packages
                else "go test ./..."
            ),
            "pytest": (
                "pytest -v integration-tests"
                if run_full_integration
                else (
                    "pytest -v " + " ".join(sorted(selected_pytest_files))
                    if selected_pytest_files
                    else "pytest -v integration-tests/test_version.py"
                )
            ),
        },
    }

    with open(args.output_json, "w", encoding="utf-8") as file:
        json.dump(selection, file, indent=2, sort_keys=True)
        file.write("\n")

    if args.github_output:
        write_github_outputs(args.github_output, selection)

    print(json.dumps(selection, indent=2, sort_keys=True))
    return 0


if __name__ == "__main__":
    sys.exit(main())
