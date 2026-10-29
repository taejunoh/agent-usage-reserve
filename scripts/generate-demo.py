#!/usr/bin/env python3
"""Generate a small terminal-style GIF from real codex-usage-reserve output."""

from __future__ import annotations

import argparse
import json
import os
import shutil
import subprocess
import tempfile
from pathlib import Path

from PIL import Image, ImageDraw, ImageFont

ROOT = Path(__file__).resolve().parents[1]
OUT = ROOT / "docs" / "assets" / "demo.gif"
TRANSCRIPT = ROOT / "docs" / "assets" / "demo-transcript.json"
WIDTH, HEIGHT = 1100, 620
BG, PANEL, PANEL2 = "#0b1117", "#111a22", "#15212a"
TEXT, MUTED, TEAL, GREEN, AMBER = "#e6edf3", "#9fb0bf", "#5eead4", "#89e6a7", "#f4bf75"


def font(size: int, bold: bool = False):
    candidates = (
        ["/System/Library/Fonts/Menlo.ttc", "/Library/Fonts/Menlo.ttc"]
        if not bold else ["/System/Library/Fonts/Menlo.ttc", "/Library/Fonts/Menlo.ttc"]
    ) + ["/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf", "/usr/share/fonts/truetype/liberation2/LiberationMono-Regular.ttf"]
    for candidate in candidates:
        if Path(candidate).exists():
            return ImageFont.truetype(candidate, size)
    return ImageFont.load_default()


FONT = font(21)
SMALL = font(16)
TITLE = font(20)


def run(binary: str, args: list[str], cwd: Path) -> dict:
    cmd = [binary, *args, "--codex-home", "./demo"]
    result = subprocess.run(cmd, cwd=cwd, text=True, capture_output=True, check=False)
    if result.returncode != 0:
        raise RuntimeError(f"Command failed ({result.returncode}): {' '.join(cmd)}\n{result.stderr}")
    output = (result.stdout + result.stderr).rstrip("\n")
    real_cwd = Path(os.path.realpath(cwd))
    output = output.replace(str(real_cwd / "demo"), "./demo").replace(str(real_cwd), ".")
    return {"command": " ".join([Path(binary).name, *args, "--codex-home", "./demo"]), "output": output, "returncode": result.returncode}


def measure_wrap(text: str, fnt, max_width: int) -> list[str]:
    words = text.split(" ")
    lines: list[str] = []
    current = ""
    for word in words:
        candidate = word if not current else f"{current} {word}"
        if ImageDraw.Draw(Image.new("RGB", (1, 1))).textlength(candidate, font=fnt) <= max_width:
            current = candidate
        else:
            if current:
                lines.append(current)
            current = word
    if current:
        lines.append(current)
    return lines or [""]


def frame(step: int, title: str, command: str, output: str, reveal: int, phase: str) -> Image.Image:
    im = Image.new("RGB", (WIDTH, HEIGHT), BG)
    d = ImageDraw.Draw(im)
    d.rounded_rectangle((26, 24, WIDTH - 26, HEIGHT - 24), radius=18, fill=PANEL, outline="#263541", width=2)
    d.rounded_rectangle((26, 24, WIDTH - 26, 80), radius=18, fill=PANEL2)
    d.rectangle((26, 58, WIDTH - 26, 80), fill=PANEL2)
    for x, color in [(52, "#ff6b6b"), (76, "#ffd166"), (100, "#06d6a0")]:
        d.ellipse((x, 46, x + 12, 58), fill=color)
    d.text((130, 39), "Codex Usage Reserve  /  real CLI session", font=SMALL, fill=MUTED)
    d.text((52, 99), "ACTUAL CLI OUTPUT · ISOLATED DEMO PROFILE", font=SMALL, fill=TEAL)
    d.text((52, 128), title, font=TITLE, fill=TEXT)
    d.text((1010, 101), f"{step}/5", font=SMALL, fill=MUTED)
    d.line((52, 163, WIDTH - 52, 163), fill="#263541", width=1)

    left, right = 56, WIDTH - 56
    y = 190
    d.text((left, y), "$", font=FONT, fill=TEAL)
    command_lines = measure_wrap(command, FONT, right - left - 34)
    for i, line in enumerate(command_lines):
        d.text((left + 28, y), line, font=FONT, fill=TEXT)
        y += 31
    y += 18
    visible = output[:reveal]
    out_lines: list[str] = []
    for raw in visible.splitlines():
        out_lines.extend(measure_wrap(raw, FONT, right - left))
    maxlines = max(1, (HEIGHT - 300) // 29)
    if len(out_lines) > maxlines:
        out_lines = ["…", *out_lines[-(maxlines - 1):]]
    for line in out_lines:
        color = GREEN if line.startswith("✓") or "enabled" in line.lower() or "enabled" == line.strip().lower() else TEXT
        if "15%" in line or "10%" in line:
            color = AMBER
        d.text((left, y), line, font=FONT, fill=color)
        y += 29
    if phase == "typing":
        cursor_y = min(y, HEIGHT - 98)
        d.rectangle((left, cursor_y, left + 12, cursor_y + 21), fill=TEAL)
    d.line((52, HEIGHT - 85, WIDTH - 52, HEIGHT - 85), fill="#263541", width=1)
    d.text((52, HEIGHT - 67), "Behavioral guidance only · not an enforced usage cap", font=SMALL, fill=MUTED)
    d.text((WIDTH - 360, HEIGHT - 67), "Paths shortened for readability", font=SMALL, fill=MUTED)
    return im


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", default=os.environ.get("CODEX_USAGE_RESERVE_DEMO_BINARY") or shutil.which("codex-usage-reserve"), help="CLI binary path (or CODEX_USAGE_RESERVE_DEMO_BINARY)")
    parser.add_argument("--preview-dir", type=Path, help="optionally save full-size review PNGs and a contact sheet here")
    args = parser.parse_args()
    if not args.binary:
        parser.error("codex-usage-reserve not found; pass --binary or set CODEX_USAGE_RESERVE_DEMO_BINARY")

    original = b"# Demo Codex instructions\n\nPrefer concise answers and include a short summary.\n"
    with tempfile.TemporaryDirectory(prefix="codex-usage-reserve-demo-") as tmp:
        cwd = Path(tmp)
        demo = cwd / "demo"
        demo.mkdir()
        (demo / "AGENTS.md").write_bytes(original)
        records = [
            run(args.binary, ["enable"], cwd),
            run(args.binary, ["set", "15"], cwd),
            run(args.binary, ["status"], cwd),
            run(args.binary, ["disable"], cwd),
        ]
        target = demo / "AGENTS.md"
        if target.read_bytes() != original:
            raise RuntimeError("Original instruction file was not restored byte-for-byte after disable")
        cat = subprocess.run(["cat", "./demo/AGENTS.md"], cwd=cwd, text=True, capture_output=True, check=False)
        if cat.returncode != 0 or cat.stdout.encode() != original:
            raise RuntimeError("Original instruction file was not restored byte-for-byte after disable")
        records.append({"command": "cat ./demo/AGENTS.md", "output": cat.stdout.rstrip("\n"), "returncode": cat.returncode})

    TRANSCRIPT.parent.mkdir(parents=True, exist_ok=True)
    TRANSCRIPT.write_text(json.dumps({"source": "captured from installed codex-usage-reserve CLI", "steps": records}, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")

    titles = ["Enable the default 10% reserve", "Change the threshold to 15%", "Review the configured policy", "Disable the policy", "Confirm original instructions remain"]
    first = records[0]
    scenes = [(1, titles[0], first["command"], first["output"], len(first["output"]), "hold", 3000)]
    for i, (record, title) in enumerate(zip(records[1:], titles[1:]), start=2):
        command = record["command"]
        output = record["output"]
        # Reveal later commands with a brief typing effect, then reveal captured output.
        scenes.append((i, title, command[:12], "", 0, "typing", 120))
        for n in range(24, len(command) + 12, 12):
            scenes.append((i, title, command[:n], "", 0, "typing", 50))
        scenes.append((i, title, command, output, 0, "output", 100))
        for n in range(1, len(output.splitlines()) + 1):
            reveal = len("\n".join(output.splitlines()[:n]))
            scenes.append((i, title, command, output, reveal, "output", 120))
        scenes.append((i, title, command, output, len(output), "hold", 2500 if i == 5 else 2350))

    images: list[Image.Image] = []
    durations: list[int] = []
    for step, title, command, output, reveal, phase, duration in scenes:
        images.append(frame(step, title, command, output, reveal, phase))
        durations.append(duration)
    if args.preview_dir:
        picks = [0, len(images) // 5, 2 * len(images) // 5, 3 * len(images) // 5, 4 * len(images) // 5, len(images) - 1]
        args.preview_dir.mkdir(parents=True, exist_ok=True)
        thumb_size = (WIDTH // 2, HEIGHT // 2)
        sheet = Image.new("RGB", (WIDTH, HEIGHT // 2 * 3), BG)
        for j, idx in enumerate(picks):
            thumb = images[idx].resize(thumb_size)
            sheet.paste(thumb, ((j % 2) * thumb_size[0], (j // 2) * thumb_size[1]))
        sheet.save(args.preview_dir / "contact-sheet.png")
        images[-1].save(args.preview_dir / "final.png")
        status_idx = max(idx for idx, scene in enumerate(scenes) if scene[0] == 3 and scene[5] == "hold")
        images[status_idx].save(args.preview_dir / "status.png")

    # Quantize to a shared 96-color palette for a small, predictable GIF.
    palette_source = images[0].convert("P", palette=Image.Palette.ADAPTIVE, colors=96)
    indexed = []
    for im in images:
        q = im.quantize(palette=palette_source, dither=Image.Dither.NONE)
        indexed.append(q)
    OUT.parent.mkdir(parents=True, exist_ok=True)
    indexed[0].save(OUT, save_all=True, append_images=indexed[1:], duration=durations, loop=0, optimize=True, disposal=1)
    print(f"Wrote {OUT} ({OUT.stat().st_size} bytes, {len(images)} frames, {sum(durations) / 1000:.1f}s)")
    print(f"Transcript: {TRANSCRIPT}")
    if args.preview_dir:
        print(f"Preview: {args.preview_dir / 'contact-sheet.png'}")


if __name__ == "__main__":
    main()
