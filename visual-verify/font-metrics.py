#!/usr/bin/env python3
"""font-metrics -- read the ink profile of a string straight out of the FONT FILE.

Why this file exists (skill `clover-engine` 铁律 3: a value with no provenance
must not enter the project; 判定权三分: "参考物自带 => 参考物"):
  the question "is the thing rendered on screen actually lowercase?" cannot be
  answered by looking at the source -- the *font asset* decides it. An
  uppercase-only pixel font renders every glyph at the same height, so a string
  that looks like mixed case in the code comes out ALL CAPS on screen (that is
  exactly how the brand line `by clover-engine` renders as `BY CLOVER-ENGINE`).

  This tool measures the font itself: for each character it records the glyph's
  ink extent above the baseline (in px at the requested size), so you can see
  (a) whether short runs exist at all (x-height letters vs cap height), and
  (b) whether any glyph sinks below the baseline (descenders) -- both are
  fingerprints of a real lowercase.

  It is also the negative control for the on-screen criterion: if the font has
  no short runs, the criterion "count(runs with h <= hMax - deficit) >= N" must
  come out False, and a criterion that still passes is measuring nothing.

Usage:
  python font-metrics.py --font "candidate=C:/path/to/font.ttf" [options]
  python font-metrics.py --font fonts/a.ttf --font "b=fonts/b.ttf" --text "by clover-engine" --size 16

Options:
  --font SPEC         FONT=PATH, or just PATH (label = file name). Repeatable. Required.
  --text STR          the string to profile (default: full ASCII letter+digit set)
  --size F            pixel size to scale to (default 16.0)
  --min-run-deficit N a run is "short" when height <= hMax - N (default 3)
  --min-short-runs N  how many short runs count as "has real lowercase" (default 6)
  --json PATH         also write the machine-readable result here

Exit codes: 0 = reported, 1 = at least one font could not be read, 2 = usage error,
3 = dependency error.
"""

import argparse
import json
import os
import sys

DEFAULT_TEXT = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"


def die(code, msg):
    sys.stderr.write("font-metrics: " + msg + "\n")
    raise SystemExit(code)


try:
    from fontTools.pens.recordingPen import RecordingPen
    from fontTools.ttLib import TTFont
except ImportError as exc:  # pragma: no cover - depends on the host
    die(3, "missing dependency (%s) -- install with: python -m pip install -r requirements.txt" % exc)


def parse_font_spec(spec):
    if "=" in spec and not os.path.exists(spec):
        label, path = spec.split("=", 1)
        label, path = label.strip(), path.strip()
    else:
        label, path = os.path.splitext(os.path.basename(spec))[0], spec.strip()
    return label, path


def glyph_extent(font, ch):
    """Return (glyph_name, ymin_px, ymax_px) for ch, or (None, None, None)."""
    cmap = font.getBestCmap()
    gname = cmap.get(ord(ch))
    if gname is None:
        return None, None, None
    gs = font.getGlyphSet()
    upm = font["head"].unitsPerEm
    scale = SIZE_HOLDER[0] / float(upm)
    pen = RecordingPen()
    gs[gname].draw(pen)
    ys = []
    for _op, pts in pen.value:
        for p in pts:
            if isinstance(p, tuple) and len(p) == 2:
                ys.append(p[1])
    if not ys:
        return gname, None, None
    return gname, int(round(min(ys) * scale)), int(round(max(ys) * scale))


SIZE_HOLDER = [16.0]


def profile(label, path, text, min_run_deficit, min_short_runs):
    try:
        font = TTFont(path, fontNumber=0)
    except Exception as exc:  # noqa: BLE001 - report and keep going
        return None, str(exc)
    upm = font["head"].unitsPerEm
    try:
        family = font["name"].getDebugName(1)
    except Exception:  # noqa: BLE001
        family = None

    chars = []
    for ch in text:
        gname, ymin, ymax = glyph_extent(font, ch)
        height = None if ymin is None else (ymax - ymin + 1)
        chars.append({"char": ch, "glyph": gname, "y_min_px": ymin, "y_max_px": ymax, "height_px": height})

    heights = sorted(c["height_px"] for c in chars if c["height_px"] is not None)
    descenders = [c for c in chars if c["y_min_px"] is not None and c["y_min_px"] < 0]
    h_max = heights[-1] if heights else 0
    short = [h for h in heights if h <= h_max - min_run_deficit]
    modal_bottom = None
    bottoms = [c["y_min_px"] for c in chars if c["y_min_px"] is not None]
    if bottoms:
        modal_bottom = max(sorted(set(bottoms)), key=bottoms.count)

    return {
        "label": label,
        "path": path,
        "family": family,
        "units_per_em": int(upm),
        "size_px": SIZE_HOLDER[0],
        "text": text,
        "chars": chars,
        "heights_sorted": heights,
        "h_max_px": h_max,
        "short_runs": len(short),
        "min_run_deficit": min_run_deficit,
        "min_short_runs": min_short_runs,
        "has_real_lowercase": bool(len(short) >= min_short_runs),
        "descender_glyphs": ["%s(%s)" % (c["char"], c["y_min_px"]) for c in descenders],
        "baseline_modal_y_min_px": modal_bottom,
        "missed_chars": [c["char"] for c in chars if c["glyph"] is None],
    }, None


def build_parser():
    p = argparse.ArgumentParser(
        prog="font-metrics.py",
        description="Ink profile of a string, measured from the font file (lowercase / descender check).",
    )
    p.add_argument("--font", action="append", required=True,
                   help="FONT=PATH or PATH (repeatable)")
    p.add_argument("--text", default=DEFAULT_TEXT, help="string to profile (default: full ASCII letters + digits)")
    p.add_argument("--size", type=float, default=16.0, help="pixel size to scale to (default 16.0)")
    p.add_argument("--min-run-deficit", type=int, default=3,
                   help="a run is short when height <= hMax - N (default 3)")
    p.add_argument("--min-short-runs", type=int, default=6,
                   help="how many short runs count as real lowercase (default 6)")
    p.add_argument("--json", dest="json_path", default="", help="also write the machine-readable result here")
    return p


def main(argv=None):
    args = build_parser().parse_args(argv)
    if args.size <= 0:
        die(2, "--size must be > 0")
    SIZE_HOLDER[0] = float(args.size)

    reports = []
    failed = []
    for spec in args.font:
        label, path = parse_font_spec(spec)
        if not os.path.isfile(path):
            failed.append((label, "no such file: " + path))
            continue
        rep, err = profile(label, path, args.text, args.min_run_deficit, args.min_short_runs)
        if rep is None:
            failed.append((label, err))
        else:
            reports.append(rep)

    lines = []
    for r in reports:
        lines.append("=" * 96)
        lines.append("%s  (%s)  family=%s  unitsPerEm=%d  size=%.1fpx"
                     % (r["label"], os.path.basename(r["path"]), r["family"], r["units_per_em"], r["size_px"]))
        lines.append("  %-5s %-16s %9s %9s %9s" % ("char", "glyph", "yMin(px)", "yMax(px)", "height(px)"))
        for c in r["chars"]:
            lines.append("  %-5s %-16s %9s %9s %9s"
                         % ("' '" if c["char"] == " " else " " + c["char"], c["glyph"],
                            c["y_min_px"], c["y_max_px"], c["height_px"]))
        lines.append("  glyph-run heights (px, sorted): %s" % (r["heights_sorted"],))
        lines.append("  predicted hMax=%d ; runs shorter than hMax-%d: %d of %d"
                     % (r["h_max_px"], r["min_run_deficit"], r["short_runs"], len(r["heights_sorted"])))
        lines.append("  => criterion 'count(short runs) >= %d' : %s"
                     % (r["min_short_runs"], r["has_real_lowercase"]))
        lines.append("  baseline modal yMin = %s ; descender glyphs (yMin < 0) = %s"
                     % (r["baseline_modal_y_min_px"], r["descender_glyphs"] or "none"))
        if r["missed_chars"]:
            lines.append("  NOTE: not in cmap (skipped): %s" % ("".join(r["missed_chars"]),))
    for label, err in failed:
        lines.append("=" * 96)
        lines.append("%s  READ FAILED: %s" % (label, err))
    text = "\n".join(lines)
    if text:
        text += "\n"

    try:
        sys.stdout.reconfigure(encoding="utf-8")
    except Exception:  # pragma: no cover
        pass
    sys.stdout.write(text)

    if args.json_path:
        with open(args.json_path, "w", encoding="utf-8") as fh:
            json.dump({"fonts": reports, "failed": [{"label": l, "error": e} for l, e in failed]},
                      fh, ensure_ascii=False, indent=2, sort_keys=True)
            fh.write("\n")

    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
