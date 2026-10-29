#!/usr/bin/env python3
"""Generate a Homebrew formula from a real release checksum manifest."""
import argparse
import json
import re
from pathlib import Path

REPOSITORY = "taejunoh/agent-usage-reserve"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("version", help="release version, with or without leading v")
    parser.add_argument("checksums", type=Path, help="checksums.txt from the release build")
    parser.add_argument("output", type=Path, help="formula destination in the Homebrew tap")
    parser.add_argument(
        "--renames",
        type=Path,
        help="also write formula_renames.json (old formula name -> new) to this path",
    )
    args = parser.parse_args()

    version = args.version.removeprefix("v")
    if not re.fullmatch(r"[0-9]+\.[0-9]+\.[0-9]+", version):
        parser.error(f"invalid release version: {args.version}")

    expected = {
        f"agent-usage-reserve_{version}_{platform}.tar.gz"
        for platform in ("darwin_amd64", "darwin_arm64", "linux_amd64", "linux_arm64", "windows_amd64", "windows_arm64")
    }
    hashes = {}
    for line in args.checksums.read_text(encoding="utf-8").splitlines():
        match = re.fullmatch(r"([0-9a-fA-F]{64})\s+\*?(\S+)", line)
        if not match:
            raise SystemExit(f"invalid checksum line: {line!r}")
        digest, filename = match.groups()
        if filename in hashes:
            raise SystemExit(f"duplicate checksum entry: {filename}")
        hashes[filename] = digest.lower()
    missing = expected - hashes.keys()
    if missing:
        raise SystemExit("missing release checksums: " + ", ".join(sorted(missing)))

    formula = [
        "class AgentUsageReserve < Formula",
        '  desc "Manage a local usage-reserve policy in Codex and Claude global instructions"',
        f'  homepage "https://github.com/{REPOSITORY}"',
        f'  url "https://github.com/{REPOSITORY}/releases/download/v{version}/agent-usage-reserve_{version}_darwin_arm64.tar.gz"',
        f'  sha256 "{hashes[f"agent-usage-reserve_{version}_darwin_arm64.tar.gz"]}"',
        '  license "MIT"',
        "",
        "  on_macos do",
        "    on_arm do",
        f'      url "https://github.com/{REPOSITORY}/releases/download/v{version}/agent-usage-reserve_{version}_darwin_arm64.tar.gz"',
        f'      sha256 "{hashes[f"agent-usage-reserve_{version}_darwin_arm64.tar.gz"]}"',
        "    end",
        "    on_intel do",
        f'      url "https://github.com/{REPOSITORY}/releases/download/v{version}/agent-usage-reserve_{version}_darwin_amd64.tar.gz"',
        f'      sha256 "{hashes[f"agent-usage-reserve_{version}_darwin_amd64.tar.gz"]}"',
        "    end",
        "  end",
        "",
        "  on_linux do",
        "    on_arm do",
        f'      url "https://github.com/{REPOSITORY}/releases/download/v{version}/agent-usage-reserve_{version}_linux_arm64.tar.gz"',
        f'      sha256 "{hashes[f"agent-usage-reserve_{version}_linux_arm64.tar.gz"]}"',
        "    end",
        "    on_intel do",
        f'      url "https://github.com/{REPOSITORY}/releases/download/v{version}/agent-usage-reserve_{version}_linux_amd64.tar.gz"',
        f'      sha256 "{hashes[f"agent-usage-reserve_{version}_linux_amd64.tar.gz"]}"',
        "    end",
        "  end",
        "",
        "  def install",
        '    bin.install "agent-usage-reserve"',
        "  end",
        "",
        "  def caveats",
        "    <<~EOS",
        "      Run agent-usage-reserve enable to activate the policy.",
        "      Run agent-usage-reserve disable before uninstalling to remove it.",
        "    EOS",
        "  end",
        "",
        "  test do",
        '    ENV["CODEX_HOME"] = (testpath/"codex-home").to_s',
        '    ENV["CLAUDE_CONFIG_DIR"] = (testpath/"claude-home").to_s',
        '    (testpath/"codex-home").mkpath',
        '    (testpath/"claude-home").mkpath',
        '    system bin/"agent-usage-reserve", "enable", "--only", "all"',
        '    system bin/"agent-usage-reserve", "status"',
        '    system bin/"agent-usage-reserve", "disable"',
        "  end",
        "end",
        "",
    ]
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text("\n".join(formula), encoding="utf-8")
    if args.renames:
        args.renames.parent.mkdir(parents=True, exist_ok=True)
        args.renames.write_text(
            json.dumps({"codex-usage-reserve": "agent-usage-reserve"}, indent=2) + "\n",
            encoding="utf-8",
        )


if __name__ == "__main__":
    main()
