#!/usr/bin/env python3
"""shot-stats -- deterministic statistics over a batch of screenshots.

Why this file exists (skill `clover-engine` -> 判定权三分: "可计算 => 脚本"):
  a batch of frames is judged by NUMBERS first, and an AI only looks at the
  frames that the numbers flag. Three cheap, deterministic signals catch the
  two silent failures that a "log is all green" run never notices:

  * degenerate frame  -- the render came out as ONE colour (or almost one).
                         Symptom: the frame is a flat fill. A single number
                         (unique colour count, modal share) makes it obvious.
  * black overlay     -- a panel / pause / result screen that is *supposed* to
                         be drawn never got drawn. Measured as the share of
                         near-black pixels.
  * colour presence   -- "is the thing we expect on screen actually there, and
                         where horizontally?" Measured as the pixel count and
                         the column runs of a named probe colour inside a
                         y-band (so a HUD strip can be excluded).

Nothing here is game specific: every colour, every band and every file comes
from the command line.

Usage:
  python shot-stats.py --shots <dir> [options]
  python shot-stats.py --shots "a.png,b.png" --out report.txt
  python shot-stats.py --shots ".ai-tmp/screenshots/*.png" --color "accent=#1c8414:40:0.15:0.80"

Options:
  --shots SPEC        directory, or a comma/semicolon separated list of paths
                      and globs (repeatable). Required.
  --out PATH          write the report here (default: stdout)
  --dark-threshold N  a pixel is "dark" when R+G+B < N (default 120)
  --min-run N         only report colour runs of at least N px (default 3)
  --max-runs N        cap the reported runs per shot (default 12)
  --color SPEC        named colour probe, repeatable:
                        NAME=#RRGGBB[:TOL[:Y0:Y1]]
                      TOL    per-channel sum tolerance (default 40)
                      Y0,Y1  y-band as a fraction of the height (default 0 1)
  --json PATH         also write the machine-readable result here

Exit codes: 0 = reported, 1 = at least one shot could not be read, 2 = usage error,
3 = dependency error.
"""

import argparse
import glob
import json
import os
import sys


def die(code, msg):
    sys.stderr.write("shot-stats: " + msg + "\n")
    raise SystemExit(code)


try:
    import numpy as np
    from PIL import Image
except ImportError as exc:  # pragma: no cover - depends on the host
    die(3, "missing dependency (%s) -- install with: python -m pip install -r requirements.txt" % exc)

IMAGE_EXT = (".png", ".jpg", ".jpeg", ".bmp", ".gif", ".tga", ".webp")


# --------------------------------------------------------------------------- #
# input
# --------------------------------------------------------------------------- #
def expand_shots(specs):
    """Turn dirs / files / globs into a sorted, de-duplicated list of files."""
    out = []
    for spec in specs:
        for chunk in str(spec).replace(";", ",").split(","):
            chunk = chunk.strip()
            if chunk == "":
                continue
            if os.path.isdir(chunk):
                for name in sorted(os.listdir(chunk)):
                    if name.lower().endswith(IMAGE_EXT):
                        out.append(os.path.join(chunk, name))
                continue
            hits = sorted(glob.glob(chunk))
            if not hits and os.path.isfile(chunk):
                hits = [chunk]
            if not hits:
                die(2, "no file matched: " + chunk)
            out.extend(hits)
    seen = set()
    uniq = []
    for p in out:
        key = os.path.abspath(p).lower()
        if key in seen:
            continue
        seen.add(key)
        uniq.append(p)
    return uniq


def parse_hex(text):
    t = text.strip().lstrip("#")
    if len(t) != 6:
        die(2, "colour must be #RRGGBB, got: " + text)
    try:
        return np.array([int(t[0:2], 16), int(t[2:4], 16), int(t[4:6], 16)], dtype=np.int32)
    except ValueError:
        die(2, "colour must be #RRGGBB, got: " + text)


def parse_color_probe(spec):
    if "=" not in spec:
        die(2, 'colour probe must look like NAME=#RRGGBB[:TOL[:Y0:Y1]], got: ' + spec)
    name, rest = spec.split("=", 1)
    parts = rest.split(":")
    rgb = parse_hex(parts[0])
    tol = int(parts[1]) if len(parts) >= 2 and parts[1] != "" else 40
    y0 = float(parts[2]) if len(parts) >= 3 and parts[2] != "" else 0.0
    y1 = float(parts[3]) if len(parts) >= 4 and parts[3] != "" else 1.0
    if y1 <= y0:
        die(2, "colour probe %s: Y1 must be > Y0" % name)
    return {"name": name.strip(), "rgb": rgb, "tol": tol, "y0": y0, "y1": y1}


# --------------------------------------------------------------------------- #
# statistics
# --------------------------------------------------------------------------- #
def runs_of(columns, min_run):
    """Contiguous column runs (start, end) of at least min_run px wide."""
    out = []
    if len(columns) == 0:
        return out
    start = columns[0]
    prev = columns[0]
    for c in columns[1:]:
        if c != prev + 1:
            if prev - start + 1 >= min_run:
                out.append((int(start), int(prev)))
            start = c
        prev = c
    if prev - start + 1 >= min_run:
        out.append((int(start), int(prev)))
    return out


def stats_for(path, probes, dark_threshold, min_run, max_runs):
    im = Image.open(path).convert("RGB")
    a = np.asarray(im, dtype=np.int16)
    h, w = a.shape[:2]
    flat = a.reshape(-1, 3)
    key = (flat[:, 0].astype(np.int64) << 16) | (flat[:, 1].astype(np.int64) << 8) | flat[:, 2]
    vals, cnt = np.unique(key, return_counts=True)
    order = np.argsort(-cnt, kind="stable")
    modal = int(vals[order[0]])
    modal_rgb = "#%02x%02x%02x" % ((modal >> 16) & 255, (modal >> 8) & 255, modal & 255)
    dark = float((flat.sum(axis=1) < dark_threshold).mean())
    row = {
        "file": os.path.basename(path),
        "path": path,
        "width": int(w),
        "height": int(h),
        "unique_colors": int(len(vals)),
        "modal_color": modal_rgb,
        "modal_share": float(cnt[order[0]] / float(len(key))),
        "dark_share": dark,
        "degenerate": bool(len(vals) <= 2 or cnt[order[0]] / float(len(key)) >= 0.995),
        "probes": [],
    }
    for p in probes:
        y0 = int(round(h * p["y0"]))
        y1 = int(round(h * p["y1"]))
        body = a[max(0, y0):min(h, y1), :]
        if body.size == 0:
            row["probes"].append({"name": p["name"], "pixels": 0, "runs": []})
            continue
        diff = np.abs(body.astype(np.int32) - p["rgb"].reshape(1, 1, 3)).sum(axis=2)
        mask = diff <= p["tol"]
        cols = np.where(mask.any(axis=0))[0]
        row["probes"].append({
            "name": p["name"],
            "pixels": int(mask.sum()),
            "column_span": [int(cols.min()), int(cols.max())] if len(cols) else None,
            "runs": runs_of(cols, min_run)[:max_runs],
            "runs_total": len(runs_of(cols, min_run)),
        })
    return row


# --------------------------------------------------------------------------- #
# main
# --------------------------------------------------------------------------- #
def build_parser():
    p = argparse.ArgumentParser(
        prog="shot-stats.py",
        description="Deterministic per-shot statistics (degenerate frame / black overlay / colour presence).",
    )
    p.add_argument("--shots", action="append", required=True,
                   help="directory, file, or glob (comma/semicolon separated; repeatable)")
    p.add_argument("--out", default="", help="write the report here (default: stdout)")
    p.add_argument("--dark-threshold", type=int, default=120,
                   help="a pixel is dark when R+G+B < N (default 120)")
    p.add_argument("--min-run", type=int, default=3, help="minimum colour run width in px (default 3)")
    p.add_argument("--max-runs", type=int, default=12, help="cap reported runs per shot (default 12)")
    p.add_argument("--color", action="append", default=[],
                   help="named colour probe NAME=#RRGGBB[:TOL[:Y0:Y1]] (repeatable)")
    p.add_argument("--json", dest="json_path", default="", help="also write the machine-readable result here")
    return p


def main(argv=None):
    args = build_parser().parse_args(argv)
    probes = [parse_color_probe(s) for s in args.color]
    shots = expand_shots(args.shots)
    if not shots:
        die(2, "--shots matched no image")

    rows = []
    unreadable = []
    for path in shots:
        try:
            rows.append(stats_for(path, probes, args.dark_threshold, args.min_run, args.max_runs))
        except Exception as exc:  # noqa: BLE001 - report and keep going
            unreadable.append((path, str(exc)))

    lines = []
    lines.append("# shot-stats: %d frame(s), %d unreadable, %d colour probe(s)"
                 % (len(shots), len(unreadable), len(probes)))
    lines.append("# %-38s %10s %6s %7s %-9s %9s %9s"
                 % ("file", "size", "colors", "modal", "modal_share", "modal%", "dark%"))
    for r in rows:
        lines.append("  %-38s %5dx%-4d %6d %-9s %11.6f %8.3f%% %8.3f%%%s"
                     % (r["file"], r["width"], r["height"], r["unique_colors"], r["modal_color"],
                        r["modal_share"], r["modal_share"] * 100.0, r["dark_share"] * 100.0,
                        "  <-- DEGENERATE FRAME" if r["degenerate"] else ""))
    for path, err in unreadable:
        lines.append("  %-38s READ FAILED: %s" % (os.path.basename(path), err))

    for p in probes:
        lines.append("")
        lines.append("# colour probe '%s' %s tol=%d y=[%.2f..%.2f]"
                     % (p["name"], "#%02x%02x%02x" % (p["rgb"][0], p["rgb"][1], p["rgb"][2]),
                        p["tol"], p["y0"], p["y1"]))
        for r in rows:
            hit = next((q for q in r["probes"] if q["name"] == p["name"]), None)
            if hit is None:
                continue
            span = "-" if hit["column_span"] is None else "%d..%d" % (hit["column_span"][0], hit["column_span"][1])
            lines.append("  %-38s pixels=%-8d span=%-12s runs=%s"
                         % (r["file"], hit["pixels"], span, hit["runs"]))
    text = "\n".join(lines)
    if text and not text.endswith("\n"):
        text += "\n"

    if args.out:
        out_dir = os.path.dirname(os.path.abspath(args.out))
        if out_dir:
            os.makedirs(out_dir, exist_ok=True)
        with open(args.out, "w", encoding="utf-8") as fh:
            fh.write(text)
    else:
        try:
            sys.stdout.reconfigure(encoding="utf-8")
        except Exception:  # pragma: no cover
            pass
        sys.stdout.write(text)

    if args.json_path:
        payload = {"shots": rows, "unreadable": [{"path": p, "error": e} for p, e in unreadable],
                   "probes": [{"name": p["name"], "tol": p["tol"], "y0": p["y0"], "y1": p["y1"]} for p in probes],
                   "dark_threshold": args.dark_threshold}
        with open(args.json_path, "w", encoding="utf-8") as fh:
            json.dump(payload, fh, ensure_ascii=False, indent=2, sort_keys=True)
            fh.write("\n")

    return 1 if unreadable else 0


if __name__ == "__main__":
    sys.exit(main())
