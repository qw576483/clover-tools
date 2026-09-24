#!/usr/bin/env python3
"""ab-region-diff -- A/B differ that NAMES the regions that differ.

Why this file exists (skill `clover-engine` -> reference/visual-loop.md):
  visual-diff.py answers the baseline question -- "does OUR frame match the
  reference (PASS/FAIL)?" -- with whole-frame numbers. This file answers the other
  question the visual loop keeps asking when both sides are OURS (before/after a UI
  move, animation frame N vs N+1, 1080p vs 720p capture): "WHICH part of the frame
  changed, and by how much?". It slices both sides into a grid of NAMED regions
  (R{row}C{col}) and prints a reproducible per-region verdict plus a region TSV,
  so the finding can be quoted and re-run -- never "looks different around the HUD".

Alignment caliber (printed on every run, never implied):
  --align resize  (default) both sides are brought to B's size with LANCZOS; the
                  source sizes and the exact scale factors are in the report;
  --align none    the sizes must match exactly, otherwise usage error (exit 2).
Anti-aliasing caliber:
  --aa-edge-band N  drop diff pixels that sit within N px of an edge in B
                  (edge = neighbouring-pixel RGB distance above --edge-threshold).
                  Documented heuristic in the same spirit as visual-diff.py's
                  --aa-policy suppress: use it for triage, and quote
                  --aa-edge-band 0 when you write down a verdict.
Diff metric: per-pixel YIQ distance -- the same formulation and constants as
  visual-diff.py (0.5053*dy^2 + 0.299*di^2 + 0.1957*dq^2, cut at 35215*threshold^2).
  Kept inline on purpose: this file must not require an edit to visual-diff.py.

Nothing here is game specific: grid, colours and thresholds all come from the CLI.
Any project with before/after screenshots can reuse it.

Usage:
  python ab-region-diff.py --a before.png --b after.png --out <dir> [options]

Options:
  --a PATH / --b PATH    the two frames (B is the "after" side and the canvas)
  --out DIR              receives ab-diff.png / ab-regions.png / ab-report.txt / ab-regions.tsv
  --grid RxC             regions per axis, e.g. 4x6 (default 4x6)
  --threshold F          per-pixel YIQ threshold, same scale as visual-diff.py (default 0.1)
  --max-total-ratio F    max differing-pixel ratio over the whole frame (default 0.001)
  --max-region-ratio F   max differing-pixel ratio inside a single region (default 0.005)
  --max-diff-pixels N    absolute cap on differing pixels, -1 = off (default -1)
  --align resize|none    size handling (default resize)
  --aa-edge-band N       ignore diffs within N px of an edge in B (default 0 = off)
  --edge-threshold N     RGB distance at which a pixel is an edge (default 60)
  --json PATH            also write the machine-readable result here

Exit codes: 0 = PASS, 1 = FAIL (a region or the whole frame is over tolerance),
2 = usage error, 3 = dependency error.
"""

import argparse
import json
import os
import sys

YIQ_Y = (0.29889531, 0.58662247, 0.11448223)
YIQ_I = (0.59597799, -0.27417610, -0.32180189)
YIQ_Q = (0.21147017, -0.52261711, 0.31114694)
PIXELMATCH_SCALE = 35215.0


def die(code, msg):
    sys.stderr.write("ab-region-diff: " + msg + "\n")
    raise SystemExit(code)


try:
    import numpy as np
    from PIL import Image
except ImportError as exc:  # pragma: no cover - depends on the host
    die(3, "missing dependency (%s) -- install with: python -m pip install -r requirements.txt" % exc)


# --------------------------------------------------------------------------- #
# metric (identical to visual-diff.py)
# --------------------------------------------------------------------------- #
def color_delta(a, b):
    """Per-pixel YIQ distance between two (H, W, 3) float arrays -> (H, W)."""
    def to_yiq(x):
        r, g, bl = x[..., 0], x[..., 1], x[..., 2]
        return (r * YIQ_Y[0] + g * YIQ_Y[1] + bl * YIQ_Y[2],
                r * YIQ_I[0] + g * YIQ_I[1] + bl * YIQ_I[2],
                r * YIQ_Q[0] + g * YIQ_Q[1] + bl * YIQ_Q[2])
    ya, ia, qa = to_yiq(a)
    yb, ib, qb = to_yiq(b)
    dy, di, dq = ya - yb, ia - ib, qa - qb
    return 0.5053 * dy * dy + 0.299 * di * di + 0.1957 * dq * dq


def edge_mask(a, threshold):
    """True where the RGB distance to a 4-neighbour exceeds threshold."""
    g = np.zeros(a.shape[:2], dtype=np.float64)
    dy = np.abs(np.diff(a, axis=0)).sum(axis=2)
    g[:-1, :] = np.maximum(g[:-1, :], dy)
    g[1:, :] = np.maximum(g[1:, :], dy)
    dx = np.abs(np.diff(a, axis=1)).sum(axis=2)
    g[:, :-1] = np.maximum(g[:, :-1], dx)
    g[:, 1:] = np.maximum(g[:, 1:], dx)
    return g > threshold


def dilate(mask, n):
    """Grow mask by n px in the 4 directions (no wrap-around)."""
    out = mask
    for _ in range(max(0, n)):
        cur = out
        acc = cur.copy()
        acc[1:, :] |= cur[:-1, :]
        acc[:-1, :] |= cur[1:, :]
        acc[:, 1:] |= cur[:, :-1]
        acc[:, :-1] |= cur[:, 1:]
        out = acc
    return out


# --------------------------------------------------------------------------- #
# grid
# --------------------------------------------------------------------------- #
def parse_grid(text):
    parts = str(text).lower().split("x")
    if len(parts) != 2:
        die(2, "--grid must look like RxC, e.g. 4x6, got: " + str(text))
    try:
        rows, cols = int(parts[0]), int(parts[1])
    except ValueError:
        die(2, "--grid must look like RxC, e.g. 4x6, got: " + str(text))
    if rows < 1 or cols < 1:
        die(2, "--grid axes must be >= 1")
    return rows, cols


def region_bounds(size, n, i):
    lo = i * size // n
    hi = (i + 1) * size // n
    return lo, hi


def bbox_of(mask):
    ys, xs = np.where(mask)
    if len(xs) == 0:
        return None
    return [int(xs.min()), int(ys.min()), int(xs.max()), int(ys.max())]


# --------------------------------------------------------------------------- #
# main
# --------------------------------------------------------------------------- #
def build_parser():
    p = argparse.ArgumentParser(
        prog="ab-region-diff.py",
        description="A/B pixel differ that reports WHICH named regions differ (before/after shots).",
    )
    p.add_argument("--a", required=True, help="the A frame (.png)")
    p.add_argument("--b", required=True, help="the B frame (.png); B is the canvas that gets annotated")
    p.add_argument("--out", required=True, help="directory that receives the report / region TSV / annotated PNGs")
    p.add_argument("--grid", default="4x6", help="regions per axis, RxC (default 4x6)")
    p.add_argument("--threshold", type=float, default=0.1, help="per-pixel YIQ threshold (default 0.1)")
    p.add_argument("--max-total-ratio", type=float, default=0.001, help="max whole-frame diff ratio (default 0.001)")
    p.add_argument("--max-region-ratio", type=float, default=0.005, help="max per-region diff ratio (default 0.005)")
    p.add_argument("--max-diff-pixels", type=int, default=-1, help="absolute cap on diff pixels, -1 = off (default -1)")
    p.add_argument("--align", choices=("resize", "none"), default="resize", help="size handling (default resize)")
    p.add_argument("--aa-edge-band", type=int, default=0, help="ignore diffs within N px of an edge in B (default 0)")
    p.add_argument("--edge-threshold", type=int, default=60, help="RGB distance at which a pixel is an edge (default 60)")
    p.add_argument("--json", dest="json_path", default="", help="also write the machine-readable result here")
    return p


def main(argv=None):
    args = build_parser().parse_args(argv)
    if args.threshold <= 0:
        die(2, "--threshold must be > 0")
    if args.aa_edge_band < 0:
        die(2, "--aa-edge-band must be >= 0")
    rows, cols = parse_grid(args.grid)
    for name in (args.a, args.b):
        if not os.path.isfile(name):
            die(2, "no such file: " + name)

    im_a = Image.open(args.a).convert("RGB")
    im_b = Image.open(args.b).convert("RGB")
    size_a, size_b = im_a.size, im_b.size
    if size_a != size_b:
        if args.align == "none":
            die(2, "--align none requires equal sizes, got %dx%d (A) vs %dx%d (B)"
                % (size_a[0], size_a[1], size_b[0], size_b[1]))
        im_a = im_a.resize(size_b, Image.LANCZOS)
        scale_x = size_b[0] / float(size_a[0])
        scale_y = size_b[1] / float(size_a[1])
        align_note = "resized A -> B size (LANCZOS); scaleX=%.6f scaleY=%.6f" % (scale_x, scale_y)
    else:
        scale_x = scale_y = 1.0
        align_note = "none needed (sizes equal)"

    a = np.asarray(im_a, dtype=np.float64)
    b = np.asarray(im_b, dtype=np.float64)
    h, w = b.shape[:2]

    delta = color_delta(a, b)
    cut = PIXELMATCH_SCALE * args.threshold * args.threshold
    diff_mask = delta > cut

    aa_dropped = 0
    if args.aa_edge_band > 0 and diff_mask.any():
        band = dilate(edge_mask(b, args.edge_threshold), args.aa_edge_band)
        drop = diff_mask & band
        aa_dropped = int(drop.sum())
        diff_mask = diff_mask & ~drop

    diff_count = int(diff_mask.sum())
    total = float(w * h)
    total_ratio = diff_count / total
    total_box = bbox_of(diff_mask)

    region_rows = []
    for r in range(rows):
        y0, y1 = region_bounds(h, rows, r)
        for c in range(cols):
            x0, x1 = region_bounds(w, cols, c)
            cell = diff_mask[y0:y1, x0:x1]
            cnt = int(cell.sum())
            cell_pixels = int((y1 - y0) * (x1 - x0))
            ratio = cnt / float(cell_pixels) if cell_pixels else 0.0
            over = ratio > args.max_region_ratio
            box = bbox_of(cell)
            if box is not None:
                box = [box[0] + x0, box[1] + y0, box[2] + x0, box[3] + y0]
            region_rows.append({
                "region": "R%dC%d" % (r + 1, c + 1),
                "x0": int(x0), "y0": int(y0), "x1": int(x1), "y1": int(y1),
                "diff_pixels": cnt, "region_pixels": cell_pixels,
                "ratio": ratio, "bbox": box, "over": bool(over),
            })

    over_rows = [q for q in region_rows if q["over"]]
    checks = [
        ("total-ratio", total_ratio <= args.max_total_ratio,
         "%.6f <= %.6f" % (total_ratio, args.max_total_ratio)),
        ("region-ratio", not over_rows,
         "0 over tolerance" if not over_rows else
         "%d region(s) over %.6f: %s" % (len(over_rows), args.max_region_ratio,
                                         ", ".join(q["region"] for q in over_rows))),
        ("diff-pixels", (args.max_diff_pixels < 0) or (diff_count <= args.max_diff_pixels),
         ("%d <= %d" % (diff_count, args.max_diff_pixels)) if args.max_diff_pixels >= 0 else "cap off"),
    ]
    failed = [n for n, ok, _ in checks if not ok]
    passed = not failed

    # ---- outputs ----
    os.makedirs(args.out, exist_ok=True)
    grid_color = (0, 120, 255)
    diff_img = np.array(b, dtype=np.uint8)
    diff_img[diff_mask] = (255, 0, 0)
    for r in range(1, rows):
        y = region_bounds(h, rows, r)[0]
        diff_img[max(0, y - 1):y + 1, :] = grid_color
    for c in range(1, cols):
        x = region_bounds(w, cols, c)[0]
        diff_img[:, max(0, x - 1):x + 1] = grid_color
    Image.fromarray(diff_img).save(os.path.join(args.out, "ab-diff.png"))

    regions_img = (b * 0.35).astype(np.uint8)
    for q in over_rows:
        regions_img[q["y0"]:q["y1"], q["x0"]:q["x1"]] = (
            regions_img[q["y0"]:q["y1"], q["x0"]:q["x1"]].astype(np.int16)
            + np.array([160, 0, 0], dtype=np.int16)).clip(0, 255).astype(np.uint8)
    for r in range(1, rows):
        y = region_bounds(h, rows, r)[0]
        regions_img[max(0, y - 1):y + 1, :] = (70, 70, 70)
    for c in range(1, cols):
        x = region_bounds(w, cols, c)[0]
        regions_img[:, max(0, x - 1):x + 1] = (70, 70, 70)
    Image.fromarray(regions_img).save(os.path.join(args.out, "ab-regions.png"))

    lines = []
    lines.append("# ab-region-diff: A=%s (%dx%d)  B=%s (%dx%d)"
                 % (args.a, size_a[0], size_a[1], args.b, size_b[0], size_b[1]))
    lines.append("# alignment: %s" % align_note)
    lines.append("# caliber: threshold=%.3f (delta > %.3f counts) | grid=%dx%d (%d regions, %dx%d px cells) | "
                 "aa_edge_band=%d edge_threshold=%d%s"
                 % (args.threshold, cut, rows, cols, rows * cols,
                    region_rows[0]["x1"] - region_rows[0]["x0"] if region_rows else 0,
                    region_rows[0]["y1"] - region_rows[0]["y0"] if region_rows else 0,
                    args.aa_edge_band, args.edge_threshold,
                    "" if aa_dropped == 0 else " (%d px dropped as anti-aliasing)" % aa_dropped))
    lines.append("# totals: diff_pixels=%d / %d = %.4f%%  diff_bbox=%s"
                 % (diff_count, int(total), total_ratio * 100.0,
                    "-" if total_box is None else "%d,%d-%d,%d" % tuple(total_box)))
    lines.append("# criteria: max_total_ratio=%.6f  max_region_ratio=%.6f  max_diff_pixels=%s"
                 % (args.max_total_ratio, args.max_region_ratio,
                    "off" if args.max_diff_pixels < 0 else str(args.max_diff_pixels)))
    for name, ok, detail in checks:
        lines.append("  check  %-13s %-4s %s" % (name, "PASS" if ok else "FAIL", detail))
    lines.append("# regions over tolerance (named, sorted by ratio):")
    if not over_rows:
        lines.append("  (none)")
    for q in sorted(over_rows, key=lambda q: (-q["ratio"], q["region"])):
        lines.append("  %-6s x=[%4d..%-4d) y=[%4d..%-4d) diff=%6d ratio=%.6f bbox=%s  FAIL  <-- fix here"
                     % (q["region"], q["x0"], q["x1"], q["y0"], q["y1"], q["diff_pixels"], q["ratio"],
                        "%d,%d-%d,%d" % tuple(q["bbox"]) if q["bbox"] else "-"))
    lines.append("# regions with any difference (sorted by ratio):")
    shown = 0
    for q in sorted([q for q in region_rows if q["diff_pixels"] > 0],
                    key=lambda q: (-q["ratio"], q["region"])):
        lines.append("  %-6s x=[%4d..%-4d) y=[%4d..%-4d) diff=%6d ratio=%.6f"
                     % (q["region"], q["x0"], q["x1"], q["y0"], q["y1"], q["diff_pixels"], q["ratio"]))
        shown += 1
    if shown == 0:
        lines.append("  (none)")
    lines.append("result   %s" % ("PASS" if passed else "FAIL -- feed the named regions back to the implementer"))
    text = "\n".join(lines) + "\n"
    with open(os.path.join(args.out, "ab-report.txt"), "w", encoding="utf-8") as fh:
        fh.write(text)
    with open(os.path.join(args.out, "ab-regions.tsv"), "w", encoding="utf-8") as fh:
        fh.write("region\tx0\ty0\tx1\ty1\tdiff_pixels\tregion_pixels\tratio\tverdict\n")
        for q in region_rows:
            fh.write("%s\t%d\t%d\t%d\t%d\t%d\t%d\t%.6f\t%s\n"
                     % (q["region"], q["x0"], q["y0"], q["x1"], q["y1"],
                        q["diff_pixels"], q["region_pixels"], q["ratio"], "FAIL" if q["over"] else "PASS"))

    if args.json_path:
        payload = {
            "a": args.a, "b": args.b,
            "size_a": list(size_a), "size_b": list(size_b),
            "align": args.align, "scale_x": scale_x, "scale_y": scale_y,
            "grid_rows": rows, "grid_cols": cols,
            "threshold": args.threshold, "delta_cut": cut,
            "aa_edge_band": args.aa_edge_band, "aa_dropped_pixels": aa_dropped,
            "diff_pixels": diff_count, "total_pixels": int(total),
            "total_ratio": total_ratio, "diff_bbox": total_box,
            "regions": region_rows,
            "over_regions": [q["region"] for q in over_rows],
            "checks": [{"name": n, "pass": bool(ok), "detail": d} for n, ok, d in checks],
            "result": "PASS" if passed else "FAIL",
        }
        with open(args.json_path, "w", encoding="utf-8") as fh:
            json.dump(payload, fh, ensure_ascii=False, indent=2, sort_keys=True)
            fh.write("\n")

    print(text)
    return 0 if passed else 1


if __name__ == "__main__":
    try:
        sys.exit(main())
    except BrokenPipeError:  # pragma: no cover
        sys.exit(0)
