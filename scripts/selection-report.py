#!/usr/bin/env python3
"""Summarize sift's automatic selection without writing a dump file."""

import argparse
import json
import shutil
import subprocess
import sys
from collections import Counter
from pathlib import Path


UI_PREFIXES = ("internal/tui/", "internal/picker/", "internal/highlight/")


def summarize(decisions):
    selected = [item for item in decisions if item.get("selected")]
    not_selected = [item for item in decisions if not item.get("selected")]
    modes = Counter()
    roles = Counter()
    for item in selected:
        mode = item.get("mode", "unknown")
        tokens = item.get("tokens", 0)
        modes[mode] += tokens
        roles[item.get("role", "unknown")] += tokens

    return {
        "files_considered": len(decisions),
        "files_selected": len(selected),
        "selected_tokens": sum(item.get("tokens", 0) for item in selected),
        "tokens_by_mode": dict(sorted(modes.items())),
        "tokens_by_role": dict(sorted(roles.items())),
        "selected_files": [
            {
                key: item.get(key)
                for key in (
                    "path",
                    "area",
                    "mode",
                    "tokens",
                    "full_tokens",
                    "signature_tokens",
                    "role",
                    "role_confidence",
                    "score",
                    "task_relevance",
                    "reason",
                    "signals",
                )
                if key in item
            }
            for item in selected
        ],
        "not_selected_files": [
            {
                key: item.get(key)
                for key in ("path", "mode", "role", "score", "tokens", "reason")
                if key in item
            }
            for item in not_selected
        ],
    }


def main():
    parser = argparse.ArgumentParser(
        description="Report selected files and token totals without rendering a dump."
    )
    parser.add_argument("root", nargs="?", default=".", help="repository to scan (default: .)")
    parser.add_argument("--budget", type=int, default=50000, help="token budget (default: 50000)")
    parser.add_argument("--prompt", default="", help="optional task prompt used for selection")
    parser.add_argument("--sift", default="sift", help="sift executable or path to it")
    args = parser.parse_args()

    sift_arg = Path(args.sift).expanduser()
    if sift_arg.parent != Path("."):
        executable = str(sift_arg.resolve()) if sift_arg.is_file() else None
    else:
        executable = shutil.which(args.sift)
    if not executable:
        print(f"Could not find sift executable: {args.sift}", file=sys.stderr)
        return 2

    root = Path(args.root).expanduser().resolve()
    command = [
        executable,
        "select",
        ".",
        "--budget",
        str(args.budget),
        "--selection-only",
        "--selection-format",
        "json",
        "--print-selection",
        "--include-skipped",
        "--smart",
    ]
    if args.prompt:
        command.extend(["--prompt", args.prompt])

    result = subprocess.run(command, cwd=root, capture_output=True, text=True)
    if result.returncode:
        print(f"Command failed: {command!r}", file=sys.stderr)
        if result.stderr:
            print(result.stderr, file=sys.stderr, end="")
        if result.stdout:
            print(result.stdout, file=sys.stderr, end="")
        return result.returncode

    try:
        report = json.loads(result.stdout)
    except json.JSONDecodeError as error:
        print(f"Could not parse sift's JSON report: {error}", file=sys.stderr)
        if result.stdout:
            print(result.stdout[:4000], file=sys.stderr)
        return 1

    decisions = report.get("decisions", [])
    groups = {}
    for prefix in UI_PREFIXES:
        groups[prefix.rstrip("/")] = summarize(
            [item for item in decisions if item.get("path", "").startswith(prefix)]
        )
    ui_decisions = [
        item for item in decisions if item.get("path", "").startswith(UI_PREFIXES)
    ]

    output = {
        "sift_executable": str(Path(executable).resolve()),
        "root": str(root),
        "command": command,
        "summary": {
            "budget": report.get("budget"),
            "used_tokens": report.get("used_tokens"),
            "selected_files": report.get("selected_files"),
        },
        "all_files": summarize(decisions),
        "ui_packages": {
            "prefixes": list(UI_PREFIXES),
            **summarize(ui_decisions),
            "by_package": groups,
        },
        "scan_skipped": {
            "count": len(report.get("skipped", [])),
            "reasons": dict(Counter(item.get("reason", "unknown") for item in report.get("skipped", []))),
            "ui_paths": [
                item
                for item in report.get("skipped", [])
                if item.get("path", "").startswith(UI_PREFIXES)
            ],
        },
    }
    print(json.dumps(output, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
