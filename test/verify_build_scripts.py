#!/usr/bin/env python3
"""Verification harness for build.sh / build_windows.ps1 (multi-platform matrix)."""

import argparse
import os
import subprocess
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
PS1 = os.path.join(ROOT, "build_windows.ps1")
SH = os.path.join(ROOT, "build.sh")

CLI_MATRIX = [
    ("Windows", "x64", "snishaper.exe"),
    ("Windows", "x86", "snishaper.exe"),
    ("Windows", "arm64", "snishaper.exe"),
    ("Linux", "x64", "snishaper"),
    ("Linux", "arm64", "snishaper"),
    ("Darwin", "x64", "snishaper"),
    ("Darwin", "arm64", "snishaper"),
]

GUI_MATRIX = [
    ("Windows", "x64", "snishaper.exe"),
    ("Windows", "x86", "snishaper.exe"),
    ("Windows", "arm64", "snishaper.exe"),
    ("Linux", "x64", "SniShaper"),
    ("Linux", "arm64", "SniShaper"),
]


def wsl_root():
    drive = ROOT[0].lower()
    return "/mnt/" + drive + ROOT[2:].replace("\\", "/")


def run(cmd, timeout):
    proc = subprocess.run(
        cmd,
        cwd=ROOT,
        capture_output=True,
        text=True,
        encoding="utf-8",
        errors="replace",
        timeout=timeout,
    )
    return proc.returncode, proc.stdout or "", proc.stderr or ""


def powershell(args, timeout=900):
    quoted = "'" + PS1 + "'"
    return run(["powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", "& " + quoted + " " + " ".join(args)], timeout)


def bash(args, timeout=900):
    return run(["wsl.exe", "--cd", wsl_root(), "bash", "-lc", "./build.sh " + " ".join(args)], timeout)


def wsl(cmd, timeout=900):
    return run(["wsl.exe", "--cd", wsl_root(), "bash", "-lc", cmd], timeout)


def show(title, code, out, err, keep_lines=None):
    print("")
    print("=" * 78)
    print("[%s] exit=%d" % (title, code))
    print("=" * 78)
    lines = [ln for ln in out.splitlines() if ln.strip()]
    if keep_lines:
        lines = [ln for ln in lines if any(k in ln for k in keep_lines)]
    print("\n".join(lines[-40:]))
    if err.strip():
        tail = [ln for ln in err.splitlines() if ln.strip()][-10:]
        print("--- stderr ---")
        print("\n".join(tail))
    return lines


def check(title, ok, detail=""):
    mark = "PASS" if ok else "FAIL"
    print("[%s] %s%s" % (mark, title, (" - " + detail) if detail else ""))
    return ok


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--scope", choices=["params", "cli", "gui", "all"], default="params")
    ap.add_argument("--timeout", type=int, default=1800)
    args = ap.parse_args()

    for stream in (sys.stdout, sys.stderr):
        try:
            stream.reconfigure(encoding="utf-8", errors="replace")
        except AttributeError:
            pass

    results = []

    print("=" * 78)
    print("SniShaper build script verification")
    print("root=%s" % ROOT)
    print("=" * 78)

    ps_cases = [
        (["-DryRun", "-All"], ("CLI", "GUI")),
        (["-DryRun", "-Type", "cli", "-All"], ("CLI",)),
        (["-DryRun", "-CI", "-Platform", "windows", "-Arch", "arm64", "-Type", "cli,gui"], ("CLI", "GUI")),
        (["-DryRun", "-Platform", "windows,linux", "-Arch", "x64,arm64", "-Type", "cli"], ("CLI",)),
        (["-DryRun", "-Build", "windows,gui,all"], ("GUI",)),
        (["-DryRun", "-CI", "-Platform", "linux", "-Arch", "arm64"], None),
    ]
    for case, keep in ps_cases:
        code, out, err = powershell(case, args.timeout)
        lines = show("ps1 " + " ".join(case), code, out, err, keep)
        if keep and len(lines) > 0:
            got = {ln.split()[0] for ln in lines if ln.startswith(("CLI ", "GUI "))}
            if "-Type" in case and "cli" in case and "gui" not in case and "," not in case and "--type" not in case:
                results.append(check("ps1 %s filters to CLI only" % " ".join(case), got == {"CLI"}))
            if "arm64" in case and "linux" in case:
                rows = [ln for ln in lines if ln.startswith(("CLI ", "GUI "))]
                results.append(check("ps1 %s only arm64 rows" % " ".join(case),
                                     len(rows) > 0 and all("/arm64" in ln for ln in rows)))
        results.append(check("ps1 %s exit 0" % " ".join(case), code == 0, "exit=%d" % code))

    sh_cases = [
        ["--dry-run", "--all"],
        ["--dry-run", "--type", "cli", "--all"],
        ["--dry-run", "--ci", "--platform", "linux", "--arch", "arm64", "--type", "cli", "--type", "gui"],
        ["--dry-run", "--platform", "windows", "--arch", "arm64"],
        ["--dry-run", "--build", "linux,gui,all", "--silent"],
    ]
    for case in sh_cases:
        code, out, err = bash(case, args.timeout)
        lines = show("build.sh " + " ".join(case), code, out, err, ("CLI", "GUI"))
        if "--type" in case and "cli" in case and "gui" not in case:
            got = {ln.split()[0] for ln in lines if ln.startswith(("CLI ", "GUI "))}
            results.append(check("build.sh %s filters to CLI only" % " ".join(case), got == {"CLI"}))
        if "--arch arm64" in case and "--all" not in case:
            rows = [ln for ln in lines if ln.startswith(("CLI ", "GUI "))]
            results.append(check("build.sh %s only arm64 rows" % " ".join(case),
                                 len(rows) > 0 and all("/arm64" in ln for ln in rows)))
        results.append(check("build.sh %s exit 0" % " ".join(case), code == 0, "exit=%d" % code))

    if args.scope in ("cli", "all"):
        code, out, err = bash(["--ci", "--type", "cli", "--all"], args.timeout)
        show("build.sh --ci --type cli --all (real build)", code, out, err)
        results.append(check("build.sh CLI matrix exit 0", code == 0, "exit=%d" % code))
        for platform, arch, name in CLI_MATRIX:
            path = os.path.join(ROOT, "build", "bin", "cli", platform, arch, name)
            results.append(check("artifact cli/%s/%s/%s" % (platform, arch, name), os.path.isfile(path)))
        code, out, err = wsl("file " + " ".join(
            os.path.join("build/bin/cli", p, a, n).replace("\\", "/") for p, a, n in CLI_MATRIX), args.timeout)
        show("file(1) on CLI artifacts", code, out, err)
        expect = {
            ("Windows", "x64"): "x86-64",
            ("Windows", "x86"): "i386",
            ("Windows", "arm64"): "ARM64",
            ("Linux", "x64"): "x86-64",
            ("Linux", "arm64"): "aarch64",
            ("Darwin", "x64"): "x86_64",
            ("Darwin", "arm64"): "arm64",
        }
        for platform, arch, _ in CLI_MATRIX:
            needle = expect[(platform, arch)]
            hit = any((("/%s/%s/" % (platform, arch)) in ln) and (needle in ln) for ln in out.splitlines())
            results.append(check("cli %s/%s is %s" % (platform, arch, needle), hit))

        code, out, err = powershell(["-CI", "-Platform", "windows", "-Arch", "x64", "-Type", "cli"], args.timeout)
        show("ps1 -CI -Platform windows -Arch x64 -Type cli (real build)", code, out, err)
        results.append(check("ps1 CLI windows/x64 exit 0", code == 0, "exit=%d" % code))
        path = os.path.join(ROOT, "build", "bin", "cli", "Windows", "x64", "snishaper.exe")
        results.append(check("artifact build/bin/cli/Windows/x64/snishaper.exe", os.path.isfile(path)))

    if args.scope in ("gui", "all"):
        code, out, err = powershell(["-CI", "-Platform", "windows", "-Arch", "x64", "-Type", "gui"], args.timeout)
        show("ps1 -CI -Platform windows -Arch x64 -Type gui (real build)", code, out, err)
        results.append(check("ps1 GUI windows/x64 exit 0", code == 0, "exit=%d" % code))
        for platform, arch, name in [("Windows", "x64", "snishaper.exe")]:
            path = os.path.join(ROOT, "build", "bin", "gui", platform, arch, name)
            results.append(check("artifact gui/%s/%s/%s" % (platform, arch, name), os.path.isfile(path)))

    print("")
    print("=" * 78)
    ok = sum(1 for r in results if r)
    bad = len(results) - ok
    print("RESULT: %d passed, %d failed" % (ok, bad))
    print("=" * 78)
    return 1 if bad else 0


if __name__ == "__main__":
    sys.exit(main())
