#!/usr/bin/env python3
"""editor-overlay-probe -- measure the EDITOR OVERLAY layer of a window capture.

Why this file exists (skill `clover-engine` -> reference/visual-loop.md, 表现类):
  What the user actually sees inside the editor is the game/back buffer PLUS a
  layer the renderer never wrote: alignment guides, grid lines, bounding boxes,
  gizmo marks, selection outlines. `ScreenCapture` / `ReadPixels` cannot see that
  layer at all, so "the guide is there" / "the box really wraps the object"
  cannot be judged from an ordinary frame screenshot.

  Pair this file with `capture-editor-window.ps1` (window-level, OS PrintWindow):
  that script produces the pixels, this one turns them into NUMBERS plus a mask,
  so two runs are comparable ("the same 3 guides at the same columns") instead of
  "looks right". It must not be confused with shot-stats.py: shot-stats answers
  "is a colour present anywhere in a band", this file answers "where are the
  primeritives (straight lines, boxes) and how thick are they".

  Nothing here is game specific -- every colour, span and threshold comes from
  the command line. Any project that can grab a window/element screenshot can
  reuse it.

Two ways to isolate the overlay:
  * auto (default) -- "ink" = pixels whose RGB differs from the image's modal
    background by more than --bg-tolerance (sum |dR|+|dG|+|dB|).
  * --color NAME=#RRGGBB[:TOL] -- isolate one known overlay colour; the syntax is
    the same as shot-stats.py --color (minus the y-band), repeatable.

What it reports:
  * straight-line primitives, horizontal and vertical, grouped per thickness:
    guides / grid lines / axes / box edges (x or y, span, thickness, len);
    clusters thicker than --max-line-thickness are reported as `band`, not `line`,
    so "a big blob" is never mistaken for a guide;
  * per-colour bbox + pixel count for every --color probe (bounding boxes);
  * the raw counters a verdict can be built on: primitives / lines_h / lines_v /
    bands_h / bands_v / ink_pixels / ink_bbox.

Usage:
  python editor-overlay-probe.py <window.png> [options]

Options:
  --out PATH             write the report here (default: stdout)
  --mask PATH            write the ink mask (white = overlay, black = the rest)
  --annotate PATH        write the capture dimmed, with detected primitives painted
                         (horizontal = red, vertical = blue, colour-probe bbox = yellow)
  --color SPEC           NAME=#RRGGBB[:TOL] overlay colour probe (repeatable)
  --bg #RRGGBB           use this colour as the background instead of the modal colour
  --bg-tolerance N       sum |dR|+|dG|+|dB| at which a pixel still counts as background
                         (default 30)
  --min-line-span N      a run must span at least N px to be a primitive (default 24)
  --max-line-thickness N a cluster at most this thick counts as a `line` (default 6)
  --max-lines N          cap primitives printed per orientation (default 20)
  --min-primitives N     exit 1 when fewer than N `line` primitives were found
                         (default 0 = report only)
  --json PATH            also write the machine-readable result here

Exit codes: 0 = reported, 1 = fewer primitives than --min-primitives,
2 = usage error, 3 = dependency error.
"""

import argparse
import json
import os
import sys


def die(code, msg):
    sys.stderr.write("editor-overlay-probe: " + msg + "\n")
    raise SystemExit(code)


try:
    import numpy as np
    from PIL import Image
except ImportError as exc:  # pragma: no cover - depends on the host
    die(3, "missing dependency (%s) -- install with: python -m pip install -r requirements.txt" % exc)


# --------------------------------------------------------------------------- #
# input
# --------------------------------------------------------------------------- #
def parse_hex(text):
    t = str(text).strip().lstrip("#")
    if len(t) != 6:
        die(2, "colour must be #RRGGBB, got: " + str(text))
    try:
        return np.array([int(t[0:2], 16), int(t[2:4], 16), int(t[4:6], 16)], dtype=np.int32)
    except ValueError:
        die(2, "colour must be #RRGGBB, got: " + str(text))


def parse_color_probe(spec):
    if "=" not in spec:
        die(2, "colour probe must look like NAME=#RRGGBB[:TOL], got: " + spec)
    name, rest = spec.split("=", 1)
    parts = rest.split(":")
    rgb = parse_hex(parts[0])
    tol = int(parts[1]) if len(parts) >= 2 and parts[1] != "" else 40
    if tol < 0:
        die(2, "colour probe %s: TOL must be >= 0" % name)
    return {"name": name.strip(), "rgb": rgb, "tol": tol}


def load_rgb(path):
    return np.asarray(Image.open(path).convert("RGB"), dtype=np.int16)


def modal_color(a):
    flat = a.reshape(-1, 3)
    key = (flat[:, 0].astype(np.int64) << 16) | (flat[:, 1].astype(np.int64) << 8) | flat[:, 2]
    vals, cnt = np.unique(key, return_counts=True)
    v = int(vals[int(np.argmax(cnt))])
    return np.array([(v >> 16) & 255, (v >> 8) & 255, v & 255], dtype=np.int32)


def hex_of(rgb):
    return "#%02x%02x%02x" % (int(rgb[0]), int(rgb[1]), int(rgb[2]))


# --------------------------------------------------------------------------- #
# masks
# --------------------------------------------------------------------------- #
def mask_of_distance(a, rgb, tol, keep_below):
    diff = np.abs(a.astype(np.int32) - rgb.reshape(1, 1, 3)).sum(axis=2)
    return diff <= tol if keep_below else diff > tol


def bbox_of(mask):
    ys, xs = np.where(mask)
    if len(xs) == 0:
        return None
    return [int(xs.min()), int(ys.min()), int(xs.max()), int(ys.max())]


# --------------------------------------------------------------------------- #
# primitive detection (straight runs grouped by thickness)
# --------------------------------------------------------------------------- #
def runs_1d(row, min_span):
    """Contiguous True runs of at least min_span, as (start, end) inclusive."""
    out = []
    start = None
    n = len(row)
    for i in range(n):
        if row[i]:
            if start is None:
                start = i
        elif start is not None:
            if i - start >= min_span:
                out.append((start, i - 1))
            start = None
    if start is not None and n - start >= min_span:
        out.append((start, n - 1))
    return out


def group_runs(mask, min_span):
    """Group consecutive rows whose long runs overlap into single clusters."""
    groups = []
    h = mask.shape[0]
    for y in range(h):
        for x0, x1 in runs_1d(mask[y], min_span):
            hit = None
            for g in groups:
                if g["last_y"] != y - 1:
                    continue
                lo = max(g["last0"], x0)
                hi = min(g["last1"], x1)
                if hi - lo + 1 >= 0.5 * min(g["last1"] - g["last0"] + 1, x1 - x0 + 1):
                    hit = g
                    break
            if hit is None:
                groups.append({"x0": x0, "x1": x1, "y0": y, "y1": y,
                               "last0": x0, "last1": x1, "last_y": y})
            else:
                hit["x0"] = min(hit["x0"], x0)
                hit["x1"] = max(hit["x1"], x1)
                hit["y1"] = y
                hit["last0"], hit["last1"], hit["last_y"] = x0, x1, y
    return groups


def find_primitives(mask, min_span, max_thickness):
    """Return (horizontal, vertical) primitive lists. See the module docstring."""
    h_lines = []
    for g in group_runs(mask, min_span):
        thick = g["y1"] - g["y0"] + 1
        h_lines.append({
            "orient": "h",
            "span": [int(g["x0"]), int(g["x1"])],
            "pos": [int(g["y0"]), int(g["y1"])],
            "length": int(g["x1"] - g["x0"] + 1),
            "thickness": int(thick),
            "kind": "line" if thick <= max_thickness else "band",
        })
    v_lines = []
    for g in group_runs(mask.T, min_span):
        thick = g["y1"] - g["y0"] + 1
        v_lines.append({
            "orient": "v",
            "span": [int(g["x0"]), int(g["x1"])],   # original y range
            "pos": [int(g["y0"]), int(g["y1"])],    # original x range
            "length": int(g["x1"] - g["x0"] + 1),
            "thickness": int(thick),
            "kind": "line" if thick <= max_thickness else "band",
        })
    key = lambda p: (-p["length"], p["pos"][0], p["span"][0])
    return sorted(h_lines, key=key), sorted(v_lines, key=key)


def paint_lines(img, prims, color):
    for p in prims:
        if p["kind"] != "line":
            continue
        if p["orient"] == "h":
            img[p["pos"][0]:p["pos"][1] + 1, p["span"][0]:p["span"][1] + 1] = color
        else:
            img[p["span"][0]:p["span"][1] + 1, p["pos"][0]:p["pos"][1] + 1] = color


def paint_rect(img, box, color):
    x0, y0, x1, y1 = box
    img[max(0, y0):y1 + 1, max(0, x0):min(img.shape[1], x0 + 2)] = color
    img[max(0, y0):y1 + 1, max(0, x1 - 1):x1 + 1] = color
    img[max(0, y0):min(img.shape[0], y0 + 2), max(0, x0):x1 + 1] = color
    img[max(0, y1 - 1):y1 + 1, max(0, x0):x1 + 1] = color


# --------------------------------------------------------------------------- #
# main
# --------------------------------------------------------------------------- #
def build_parser():
    p = argparse.ArgumentParser(
        prog="editor-overlay-probe.py",
        description="Measure the editor overlay layer of a window capture "
                    "(guides / grid lines / boxes / gizmo marks) as numbers + a mask.",
    )
    p.add_argument("shot", help="the window capture (.png) -- e.g. from capture-editor-window.ps1")
    p.add_argument("--out", default="", help="write the report here (default: stdout)")
    p.add_argument("--mask", default="", help="write the ink mask here (white = overlay)")
    p.add_argument("--annotate", default="", help="write the capture dimmed with primitives painted here")
    p.add_argument("--color", action="append", default=[],
                   help="overlay colour probe NAME=#RRGGBB[:TOL] (repeatable)")
    p.add_argument("--bg", default="", help="background colour #RRGGBB (default: the modal colour)")
    p.add_argument("--bg-tolerance", type=int, default=30,
                   help="sum |dR|+|dG|+|dB| still counted as background (default 30)")
    p.add_argument("--min-line-span", type=int, default=24,
                   help="minimum run length in px to be a primitive (default 24)")
    p.add_argument("--max-line-thickness", type=int, default=6,
                   help="clusters at most this thick count as a line (default 6)")
    p.add_argument("--max-lines", type=int, default=20,
                   help="cap primitives printed per orientation (default 20)")
    p.add_argument("--min-primitives", type=int, default=0,
                   help="exit 1 when fewer than N line primitives were found (default 0)")
    p.add_argument("--json", dest="json_path", default="", help="also write the machine-readable result here")
    return p


def main(argv=None):
    args = build_parser().parse_args(argv)
    if not os.path.isfile(args.shot):
        die(2, "no such file: " + args.shot)
    if args.min_line_span < 1:
        die(2, "--min-line-span must be >= 1")
    if args.max_line_thickness < 1:
        die(2, "--max-line-thickness must be >= 1")
    if args.bg_tolerance < 0:
        die(2, "--bg-tolerance must be >= 0")

    probes = [parse_color_probe(s) for s in args.color]
    a = load_rgb(args.shot)
    h, w = a.shape[:2]

    bg = parse_hex(args.bg) if args.bg else modal_color(a)
    ink = mask_of_distance(a, bg, args.bg_tolerance, keep_below=False)
    ink_pixels = int(ink.sum())
    ink_box = bbox_of(ink)

    h_lines, v_lines = find_primitives(ink, args.min_line_span, args.max_line_thickness)
    lines_h = [p for p in h_lines if p["kind"] == "line"]
    lines_v = [p for p in v_lines if p["kind"] == "line"]
    bands_h = [p for p in h_lines if p["kind"] == "band"]
    bands_v = [p for p in v_lines if p["kind"] == "band"]
    primitives = len(lines_h) + len(lines_v)

    probe_rows = []
    for p in probes:
        m = mask_of_distance(a, p["rgb"], p["tol"], keep_below=True)
        ph, pv = find_primitives(m, args.min_line_span, args.max_line_thickness)
        probe_rows.append({
            "name": p["name"], "rgb": hex_of(p["rgb"]), "tol": p["tol"],
            "pixels": int(m.sum()), "bbox": bbox_of(m),
            "primitives": len([q for q in ph + pv if q["kind"] == "line"]),
        })

    lines = []
    lines.append("# editor-overlay-probe: %s  %dx%d" % (os.path.basename(args.shot), w, h))
    lines.append("# bg=%s (source: %s)  bg_tolerance=%d  ink_pixels=%d (%.4f%%)  ink_bbox=%s"
                 % (hex_of(bg), "cli" if args.bg else "modal", args.bg_tolerance, ink_pixels,
                    ink_pixels / float(w * h) * 100.0,
                    "-" if ink_box is None else "%d,%d-%d,%d" % (ink_box[0], ink_box[1], ink_box[2], ink_box[3])))
    lines.append("# caliber: min_line_span=%d  max_line_thickness=%d  (thicker clusters are `band`, not `line`)"
                 % (args.min_line_span, args.max_line_thickness))
    for pr in probe_rows:
        lines.append("# colour probe '%s' %s tol=%d pixels=%d bbox=%s primitives=%d"
                     % (pr["name"], pr["rgb"], pr["tol"], pr["pixels"],
                        "-" if pr["bbox"] is None else "%d,%d-%d,%d" % tuple(pr["bbox"]), pr["primitives"]))
    lines.append("")
    lines.append("HORIZONTAL primitives: %d line(s), %d band(s)  (sorted by length)"
                 % (len(lines_h), len(bands_h)))
    for p in h_lines[:args.max_lines]:
        lines.append("  h  y=%4d..%-4d x=%4d..%-4d len=%5d thick=%3d  %s"
                     % (p["pos"][0], p["pos"][1], p["span"][0], p["span"][1],
                        p["length"], p["thickness"], p["kind"]))
    if len(h_lines) > args.max_lines:
        lines.append("  ... %d more (raise --max-lines)" % (len(h_lines) - args.max_lines))
    lines.append("VERTICAL primitives: %d line(s), %d band(s)"
                 % (len(lines_v), len(bands_v)))
    for p in v_lines[:args.max_lines]:
        lines.append("  v  x=%4d..%-4d y=%4d..%-4d len=%5d thick=%3d  %s"
                     % (p["pos"][0], p["pos"][1], p["span"][0], p["span"][1],
                        p["length"], p["thickness"], p["kind"]))
    if len(v_lines) > args.max_lines:
        lines.append("  ... %d more (raise --max-lines)" % (len(v_lines) - args.max_lines))
    lines.append("")
    lines.append("summary  primitives=%d lines_h=%d lines_v=%d bands_h=%d bands_v=%d ink_pixels=%d"
                 % (primitives, len(lines_h), len(lines_v), len(bands_h), len(bands_v), ink_pixels))
    ok = primitives >= args.min_primitives
    lines.append("result   %s (--min-primitives %d)" % ("PASS" if ok else "FAIL", args.min_primitives))
    text = "\n".join(lines) + "\n"

    if args.mask:
        out_dir = os.path.dirname(os.path.abspath(args.mask))
        if out_dir:
            os.makedirs(out_dir, exist_ok=True)
        Image.fromarray(np.where(ink, 255, 0).astype(np.uint8)).save(args.mask)

    if args.annotate:
        out_dir = os.path.dirname(os.path.abspath(args.annotate))
        if out_dir:
            os.makedirs(out_dir, exist_ok=True)
        ann = (a.astype(np.float64) * 0.55).astype(np.uint8)
        paint_lines(ann, h_lines, (255, 0, 0))
        paint_lines(ann, v_lines, (0, 160, 255))
        for pr in probe_rows:
            if pr["bbox"]:
                paint_rect(ann, pr["bbox"], (255, 220, 0))
        Image.fromarray(ann).save(args.annotate)

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
        payload = {
            "shot": args.shot, "width": int(w), "height": int(h),
            "background": hex_of(bg), "background_source": "cli" if args.bg else "modal",
            "bg_tolerance": args.bg_tolerance,
            "ink_pixels": ink_pixels, "ink_bbox": ink_box,
            "min_line_span": args.min_line_span,
            "max_line_thickness": args.max_line_thickness,
            "horizontal": h_lines, "vertical": v_lines,
            "probes": probe_rows,
            "primitives": primitives,
            "lines_h": len(lines_h), "lines_v": len(lines_v),
            "bands_h": len(bands_h), "bands_v": len(bands_v),
            "min_primitives": args.min_primitives,
            "result": "PASS" if ok else "FAIL",
        }
        with open(args.json_path, "w", encoding="utf-8") as fh:
            json.dump(payload, fh, ensure_ascii=False, indent=2, sort_keys=True)
            fh.write("\n")

    return 0 if ok else 1


if __name__ == "__main__":
    try:
        sys.exit(main())
    except BrokenPipeError:  # pragma: no cover
        sys.exit(0)
