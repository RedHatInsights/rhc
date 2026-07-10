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


def load_go_package_roots(config: dict[str, Any]) -> set[str]:
    configured = config.get("go_package_roots")
    if configured is None:
        raise SystemExit(
            "Missing required 'go_package_roots' in test map configuration."
        )
    if not isinstance(configured, list) or not all(
        isinstance(item, str) and item.strip() for item in configured
    ):
        raise SystemExit(
            "'go_package_roots' must be a non-empty list of non-empty strings."
        )
    return {item.strip() for item in configured}


def normalize_go_package(path: str, go_package_roots: set[str]) -> str | None:
    p = Path(path)

    if p.suffix != ".go":
        return None

    if not p.parts or p.parts[0] not in go_package_roots:
        return None

    return f"./{p.parent}"


def write_github_outputs(path: str, selection: dict[str, Any]) -> None:
    outputs = {
        "run_full_integration": "true"
        if selection["run_full_integration"]
        else "false",
        "risk_level": selection["risk_level"],
        "go_packages": ",".join(selection["selected_go_packages"]),
        "pytest_files": ",".join(selection["selected_pytest_files"]),
        "changed_files_count": len(selection["changed_files"]),
        "docs_only": "true" if selection["docs_only_change"] else "false",
    }

    with open(path, "a", encoding="utf-8") as file:
        for key, value in outputs.items():
            file.write(f"{key}={value}\n")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Select test targets based on changed files."
    )
    parser.add_argument("--base", required=True, help="Base git ref/sha")
    parser.add_argument("--head", required=True, help="Head git ref/sha")
    parser.add_argument(
        "--map-file",
        default="scripts/its_test_map.yml",
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
    return parser.parse_args()


def load_config(map_file: str) -> dict[str, Any]:
    with open(map_file, "r", encoding="utf-8") as file:
        loaded = yaml.safe_load(file)
    return loaded if isinstance(loaded, dict) else {}


def compute_considered_files(
    changed_files: list[str],
    ignore_patterns: list[re.Pattern[str]],
    docs_patterns: list[re.Pattern[str]],
) -> tuple[list[str], bool]:
    considered_files = [
        path for path in changed_files if not matches_any(path, ignore_patterns)
    ]
    docs_only_change = bool(considered_files) and all(
        matches_any(path, docs_patterns) for path in considered_files
    )
    return considered_files, docs_only_change


def apply_rule_selection(
    rules: list[dict[str, Any]],
    considered_files: list[str],
) -> tuple[set[str], set[str], list[str], list[str], dict[str, list[str]]]:
    selected_go_packages: set[str] = set()
    selected_pytest_files: set[str] = set()
    matched_rules: list[str] = []
    reasons: list[str] = []
    rule_match_by_file: dict[str, list[str]] = {path: [] for path in considered_files}

    for rule in rules:
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

    return (
        selected_go_packages,
        selected_pytest_files,
        matched_rules,
        reasons,
        rule_match_by_file,
    )


def apply_direct_and_derived_targets(
    considered_files: list[str],
    go_package_roots: set[str],
    rule_match_by_file: dict[str, list[str]],
    selected_go_packages: set[str],
    selected_pytest_files: set[str],
    reasons: list[str],
) -> None:
    for path in considered_files:
        if path.startswith("integration-tests/test_") and path.endswith(".py"):
            selected_pytest_files.add(path)
            reasons.append(f"direct-test-file:{path}")

        package = normalize_go_package(path, go_package_roots)
        if package and not rule_match_by_file[path]:
            selected_go_packages.add(package)
            reasons.append(f"derived-go-package:{package}")


def compute_full_run_decision(
    considered_files: list[str],
    fallback_patterns: list[re.Pattern[str]],
    reasons: list[str],
) -> tuple[bool, list[str]]:
    full_run_triggers = [
        path for path in considered_files if matches_any(path, fallback_patterns)
    ]
    run_full_integration = bool(full_run_triggers)

    if not considered_files:
        run_full_integration = True
        reasons.append("no-considered-files")

    return run_full_integration, full_run_triggers


def apply_smoke_or_full_policy(
    run_full_integration: bool,
    full_run_triggers: list[str],
    docs_only_change: bool,
    smoke_go: set[str],
    smoke_pytest: set[str],
    selected_go_packages: set[str],
    selected_pytest_files: set[str],
    reasons: list[str],
) -> None:
    if run_full_integration:
        reasons.extend(f"full-trigger:{path}" for path in full_run_triggers)
        return

    selected_go_packages.update(smoke_go)
    # Keep docs-only changes very cheap while still exercising basic CLI output.
    selected_pytest_files.update(smoke_pytest)
    if docs_only_change:
        reasons.append("docs-only-change")


def compute_risk_level(run_full_integration: bool, docs_only_change: bool) -> str:
    if run_full_integration:
        return "high"
    if docs_only_change:
        return "low"
    return "medium"


def build_recommended_commands(
    selected_go_packages: set[str],
    selected_pytest_files: set[str],
    run_full_integration: bool,
) -> dict[str, str]:
    return {
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
    }


def build_selection(
    args: argparse.Namespace,
    changed_files: list[str],
    considered_files: list[str],
    docs_only_change: bool,
    matched_rules: list[str],
    selected_go_packages: set[str],
    selected_pytest_files: set[str],
    run_full_integration: bool,
    risk_level: str,
    reasons: list[str],
) -> dict[str, Any]:
    return {
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
        "recommended_commands": build_recommended_commands(
            selected_go_packages,
            selected_pytest_files,
            run_full_integration,
        ),
    }


def main() -> int:
    args = parse_args()

    config = load_config(args.map_file)

    changed_files = git_changed_files(args.base, args.head)
    ignore_patterns = compile_patterns(config.get("ignore_patterns", []))
    docs_patterns = compile_patterns(config.get("docs_only_patterns", []))
    fallback_patterns = compile_patterns(config.get("fallback_patterns", []))
    go_package_roots = load_go_package_roots(config)

    considered_files, docs_only_change = compute_considered_files(
        changed_files, ignore_patterns, docs_patterns
    )

    smoke = config.get("smoke", {})
    smoke_go = set(smoke.get("go_packages", []))
    smoke_pytest = set(smoke.get("pytest_files", []))

    (
        selected_go_packages,
        selected_pytest_files,
        matched_rules,
        reasons,
        rule_match_by_file,
    ) = apply_rule_selection(config.get("rules", []), considered_files)

    apply_direct_and_derived_targets(
        considered_files,
        go_package_roots,
        rule_match_by_file,
        selected_go_packages,
        selected_pytest_files,
        reasons,
    )

    run_full_integration, full_run_triggers = compute_full_run_decision(
        considered_files, fallback_patterns, reasons
    )

    apply_smoke_or_full_policy(
        run_full_integration,
        full_run_triggers,
        docs_only_change,
        smoke_go,
        smoke_pytest,
        selected_go_packages,
        selected_pytest_files,
        reasons,
    )

    risk_level = compute_risk_level(run_full_integration, docs_only_change)
    selection = build_selection(
        args,
        changed_files,
        considered_files,
        docs_only_change,
        matched_rules,
        selected_go_packages,
        selected_pytest_files,
        run_full_integration,
        risk_level,
        reasons,
    )

    with open(args.output_json, "w", encoding="utf-8") as file:
        json.dump(selection, file, indent=2, sort_keys=True)
        file.write("\n")

    if args.github_output:
        write_github_outputs(args.github_output, selection)

    print(json.dumps(selection, indent=2, sort_keys=True))
    return 0


if __name__ == "__main__":
    sys.exit(main())
