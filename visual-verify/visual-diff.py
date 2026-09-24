#!/usr/bin/env python3
"""visual-diff -- deterministic pixel diff for the 1:1 visual loop.

Why this file exists (skill `clover-engine` -> reference/visual-loop.md):
  "pixel-perfect is a property of the RENDERED output, not of the source".
  So the verdict must be a NUMBER produced by a deterministic diff -- never
  "I looked at it and it seems fine", and never an AI looking at pictures.
  The AI may only triage/cluster what the diff found; a human makes the call.

What it writes into <out-dir>:
  side-by-side.png   baseline | ours, normalised to the same height
  diff-overlay.png   ours with every differing pixel painted red
  delta.txt          the TODO list: the verdict numbers + a per-band breakdown
                     (which horizontal slice of the frame carries the difference)

Verdict = three machine-checkable numbers, ALL of which must hold:
  diff ratio   <= --max-diff-ratio   (default 0.001, i.e. <= 0.1% of pixels)
  diff pixels  <= --max-diff-pixels  (default -1 = this cap is off)
  SSIM mean    >= --min-ssim         (default 0.99; --no-ssim makes it SKIPPED)

Diff algorithm: per-pixel YIQ distance, the formulation pixelmatch uses
  (reference/deterministic-gates.md). YIQ uses the standard NTSC weights; the
  per-pixel delta is 0.5053*dy^2 + 0.299*di^2 + 0.1957*dq^2 and a pixel counts
  as different when it exceeds 35215 * threshold^2 -- pixelmatch's scale, whose
  default threshold 0.1 corresponds to 352.15.

Anti-aliasing policy (--aa-policy):
  count     (default) every pixel counts. Honest default: an anti-aliasing
            difference is still a visible difference.
  suppress  drop diff pixels that look like anti-aliasing: the pixel sits on a
            colour boundary in OUR frame AND at least one of its 8 neighbours is
            bit-identical between the two frames. This is a documented heuristic
            in the spirit of pixelmatch's includeAA:false -- NOT a bit-exact
            port of it.
  Whatever the policy, --aa-policy count is the one to quote in a verdict; the
  suppressed run is for triage only.

SSIM: uniform 8x8 sliding window (integral image) over the grey image
  (0.299R + 0.587G + 0.114B), C1 = (0.01*L)^2, C2 = (0.03*L)^2 with L = 255 --
  the usual constants. The verdict uses the MEAN of the SSIM map; the minimum
  block value is printed for triage.

Usage:
  python visual-diff.py <baseline.png> <ours.png> <out-dir> [options]

Options:
  --height N            normalise both images to this height (default 540)
  --width N             force the normalised width (default: derived from ours)
  --threshold F         pixelmatch per-pixel threshold (default 0.1)
  --max-diff-ratio F    max differing-pixel ratio (default 0.001)
  --max-diff-pixels N   extra absolute cap on differing pixels (default -1 = off)
  --min-ssim F          minimum mean SSIM (default 0.99)
  --bands N             number of bands in the TODO list (default 8)
  --aa-policy NAME      count | suppress (default count)
  --keep-size           do not rescale; both images must already be the same size
  --no-ssim             skip SSIM (that criterion then reports SKIPPED)
  --json PATH           also write the machine-readable result to PATH

Exit codes: 0 = PASS, 1 = FAIL (over tolerance), 2 = usage error, 3 = dependency error.
"""

import argparse
import json
import os
import sys

YIQ_Y = (0.29889531, 0.58662247, 0.11448223)
YIQ_I = (0.59597799, -0.27417610, -0.32180189)
YIQ_Q = (0.21147017, -0.52261711, 0.31114694)

PIXELMATCH_SCALE = 35215.0  # a delta of 35215 == "completely different" in pixelmatch

SSIM_WINDOW = 8
SSIM_L = 255.0
SSIM_K1 = 0.01
SSIM_K2 = 0.03


def die(code, msg):
    sys.stderr.write("visual-diff: " + msg + "\n")
    raise SystemExit(code)


try:
    import numpy as np
    from PIL import Image
except ImportError as exc:  # pragma: no cover - depends on the host
    die(3, "missing dependency (%s) -- install with: python -m pip install -r requirements.txt" % exc)


# --------------------------------------------------------------------------- #
# core numbers
# --------------------------------------------------------------------------- #
def rgb_to_yiq(a):
    """a: float64 array (H, W, 3) -> three float64 arrays."""
    r, g, b = a[..., 0], a[..., 1], a[..., 2]
    y = r * YIQ_Y[0] + g * YIQ_Y[1] + b * YIQ_Y[2]
    i = r * YIQ_I[0] + g * YIQ_I[1] + b * YIQ_I[2]
    q = r * YIQ_Q[0] + g * YIQ_Q[1] + b * YIQ_Q[2]
    return y, i, q


def color_delta(a, b):
    """Per-pixel YIQ distance between two (H, W, 3) arrays -> (H, W) float64."""
    ya, ia, qa = rgb_to_yiq(a)
    yb, ib, qb = rgb_to_yiq(b)
    dy = ya - yb
    di = ia - ib
    dq = qa - qb
    return 0.5053 * dy * dy + 0.299 * di * di + 0.1957 * dq * dq


def _or_shift_neighbours(mask, dy, dx):
    """out[y+dy, x+dx] |= mask[y, x] -- no wrap-around at the borders."""
    h, w = mask.shape
    out = np.zeros_like(mask)
    yd = slice(max(0, dy), h + min(0, dy))
    ys = slice(max(0, -dy), h + min(0, -dy))
    xd = slice(max(0, dx), w + min(0, dx))
    xs = slice(max(0, -dx), w + min(0, -dx))
    out[yd, xd] |= mask[ys, xs]
    return out


def neighbour_any(mask):
    """True where any of the 8 neighbours of a pixel is True."""
    out = np.zeros_like(mask)
    for dy in (-1, 0, 1):
        for dx in (-1, 0, 1):
            if dy == 0 and dx == 0:
                continue
            out |= _or_shift_neighbours(mask, dy, dx)
    return out


def sliding_box_mean(a, size):
    """Mean over a size x size window, edge-padded (integral image, exact)."""
    pad = size // 2
    ap = np.pad(a, pad, mode="edge")
    c = np.cumsum(np.cumsum(ap, axis=0), axis=1)
    c = np.pad(c, ((1, 0), (1, 0)))
    h, w = a.shape
    s = c[size:size + h, size:size + w] - c[0:h, size:size + w] \
        - c[size:size + h, 0:w] + c[0:h, 0:w]
    return s / float(size * size)


def ssim_stats(grey_a, grey_b):
    """Return (mean_ssim, min_block_ssim) for two (H, W) grey float64 arrays."""
    w = SSIM_WINDOW
    mu_a = sliding_box_mean(grey_a, w)
    mu_b = sliding_box_mean(grey_b, w)
    mu_aa = mu_a * mu_a
    mu_bb = mu_b * mu_b
    mu_ab = mu_a * mu_b
    sig_aa = sliding_box_mean(grey_a * grey_a, w) - mu_aa
    sig_bb = sliding_box_mean(grey_b * grey_b, w) - mu_bb
    sig_ab = sliding_box_mean(grey_a * grey_b, w) - mu_ab
    c1 = (SSIM_K1 * SSIM_L) ** 2
    c2 = (SSIM_K2 * SSIM_L) ** 2
    num = (2.0 * mu_ab + c1) * (2.0 * sig_ab + c2)
    den = (mu_aa + mu_bb + c1) * (sig_aa + sig_bb + c2)
    m = num / den
    return float(m.mean()), float(m.min())


# --------------------------------------------------------------------------- #
# image loading
# --------------------------------------------------------------------------- #
def load_grey(im):
    a = np.asarray(im, dtype=np.float64)
    return a[..., 0] * 0.299 + a[..., 1] * 0.587 + a[..., 2] * 0.114


def resize_to(path, width, height, keep_size):
    im = Image.open(path).convert("RGB")
    if keep_size:
        return im
    if im.size != (width, height):
        im = im.resize((width, height), Image.LANCZOS)
    return im


# --------------------------------------------------------------------------- #
# main
# --------------------------------------------------------------------------- #
def build_parser():
    p = argparse.ArgumentParser(
        prog="visual-diff.py",
        description="Deterministic pixel diff (YIQ + SSIM) for the 1:1 visual loop.",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=__doc__.split("Usage:", 1)[1] if "Usage:" in __doc__ else None,
    )
    p.add_argument("baseline", help="baseline / reference-side PNG")
    p.add_argument("ours", help="our rendered PNG")
    p.add_argument("out_dir", help="directory that receives side-by-side.png / diff-overlay.png / delta.txt")
    p.add_argument("--height", type=int, default=540, help="normalise both images to this height (default 540)")
    p.add_argument("--width", type=int, default=0, help="force the normalised width (default: derived from ours)")
    p.add_argument("--threshold", type=float, default=0.1, help="pixelmatch per-pixel threshold (default 0.1)")
    p.add_argument("--max-diff-ratio", type=float, default=0.001, help="max differing-pixel ratio (default 0.001)")
    p.add_argument("--max-diff-pixels", type=int, default=-1, help="absolute cap on differing pixels (default -1 = off)")
    p.add_argument("--min-ssim", type=float, default=0.99, help="minimum mean SSIM (default 0.99)")
    p.add_argument("--bands", type=int, default=8, help="number of bands in the TODO list (default 8)")
    p.add_argument("--aa-policy", choices=("count", "suppress"), default="count",
                   help="count = every pixel counts (default) ; suppress = drop anti-aliasing-looking pixels")
    p.add_argument("--keep-size", action="store_true", help="do not rescale; sizes must already match")
    p.add_argument("--no-ssim", action="store_true", help="skip SSIM (criterion reports SKIPPED)")
    p.add_argument("--json", dest="json_path", default="", help="also write the machine-readable result here")
    return p


def main(argv=None):
    args = build_parser().parse_args(argv)

    if args.height <= 0:
        die(2, "--height must be > 0")
    if args.bands <= 0:
        die(2, "--bands must be > 0")
    if args.threshold <= 0:
        die(2, "--threshold must be > 0")
    for name in (args.baseline, args.ours):
        if not os.path.isfile(name):
            die(2, "no such file: " + name)

    base0 = Image.open(args.baseline).convert("RGB")
    ours0 = Image.open(args.ours).convert("RGB")

    if args.keep_size:
        if base0.size != ours0.size:
            die(2, "--keep-size requires equal sizes, got %dx%d vs %dx%d"
                % (base0.size[0], base0.size[1], ours0.size[0], ours0.size[1]))
        width, height = ours0.size
    else:
        height = args.height
        if args.width > 0:
            width = args.width
        else:
            ratio = height / float(ours0.size[1])
            width = max(16, int(round(ours0.size[0] * ratio)))

    base = np.asarray(resize_to(args.baseline, width, height, args.keep_size), dtype=np.float64)
    ours = np.asarray(resize_to(args.ours, width, height, args.keep_size), dtype=np.float64)

    delta = color_delta(base, ours)
    max_delta = PIXELMATCH_SCALE * args.threshold * args.threshold
    diff_mask = delta > max_delta

    aa_suppressed = 0
    if args.aa_policy == "suppress" and diff_mask.any():
        agree = delta == 0.0
        agree_nb = neighbour_any(agree)
        on_edge = np.zeros_like(diff_mask)
        for dy in (-1, 0, 1):
            for dx in (-1, 0, 1):
                if dy == 0 and dx == 0:
                    continue
                shifted = np.zeros_like(ours)
                h, w = diff_mask.shape
                yd = slice(max(0, dy), h + min(0, dy))
                ys = slice(max(0, -dy), h + min(0, -dy))
                xd = slice(max(0, dx), w + min(0, dx))
                xs = slice(max(0, -dx), w + min(0, -dx))
                shifted[yd, xd] = ours[ys, xs]
                on_edge |= color_delta(ours, shifted) > 0.0
        drop = diff_mask & agree_nb & on_edge
        aa_suppressed = int(drop.sum())
        diff_mask = diff_mask & ~drop

    diff_count = int(diff_mask.sum())
    total = float(width * height)
    ratio = diff_count / total

    # per-band breakdown (bands are vertical slices: left -> right)
    bands = args.bands
    xs = np.arange(width)
    band_of = np.minimum(bands - 1, xs * bands // width)
    band_hits = [int(diff_mask[:, band_of == b].sum()) for b in range(bands)]
    band_pixels = [int(height * int((band_of == b).sum())) for b in range(bands)]

    # overlay: paint the differing pixels red
    overlay = np.array(ours, dtype=np.uint8)
    overlay[diff_mask] = (255, 0, 0)
    side = Image.new("RGB", (width * 2 + 8, height), (24, 24, 24))
    side.paste(Image.fromarray(np.array(base, dtype=np.uint8)), (0, 0))
    side.paste(Image.fromarray(np.array(ours, dtype=np.uint8)), (width + 8, 0))

    if args.no_ssim:
        ssim_mean, ssim_min = None, None
    else:
        ssim_mean, ssim_min = ssim_stats(load_grey(Image.fromarray(np.array(base, dtype=np.uint8))),
                                         load_grey(Image.fromarray(np.array(ours, dtype=np.uint8))))

    checks = [
        ("diff-ratio", ratio <= args.max_diff_ratio,
         "%.6f <= %.6f" % (ratio, args.max_diff_ratio)),
        ("diff-pixels", (args.max_diff_pixels < 0) or (diff_count <= args.max_diff_pixels),
         ("%d <= %d" % (diff_count, args.max_diff_pixels)) if args.max_diff_pixels >= 0 else "cap off"),
        ("ssim-mean", True if ssim_mean is None else (ssim_mean >= args.min_ssim),
         "SKIPPED (--no-ssim)" if ssim_mean is None else "%.6f >= %.6f" % (ssim_mean, args.min_ssim)),
    ]
    failed = [name for name, ok, _ in checks if not ok]
    passed = not failed

    os.makedirs(args.out_dir, exist_ok=True)
    side.save(os.path.join(args.out_dir, "side-by-side.png"))
    Image.fromarray(overlay).save(os.path.join(args.out_dir, "diff-overlay.png"))

    lines = []
    lines.append("baseline : %s (%dx%d) -> normalised %dx%d"
                 % (args.baseline, base0.size[0], base0.size[1], width, height))
    lines.append("ours     : %s (%dx%d)" % (args.ours, ours0.size[0], ours0.size[1]))
    lines.append("diff     : %d / %d pixels = %.4f%%  (verdict number)"
                 % (diff_count, int(total), ratio * 100.0))
    lines.append("ssim     : mean=%s min-block=%s"
                 % ("SKIPPED" if ssim_mean is None else "%.6f" % ssim_mean,
                    "SKIPPED" if ssim_min is None else "%.6f" % ssim_min))
    lines.append("criteria : --threshold %.3f (delta > %.3f counts) | --max-diff-ratio %.6f | "
                 "--max-diff-pixels %s | --min-ssim %.4f | --aa-policy %s%s"
                 % (args.threshold, max_delta, args.max_diff_ratio,
                    "off" if args.max_diff_pixels < 0 else str(args.max_diff_pixels),
                    args.min_ssim, args.aa_policy,
                    ("" if aa_suppressed == 0 else " (%d px suppressed as anti-aliasing)" % aa_suppressed)))
    for name, ok, detail in checks:
        lines.append("  check  %-12s %-4s %s" % (name, "PASS" if ok else "FAIL", detail))
    lines.append("result   : %s" % ("PASS" if passed else "FAIL -- feed the bands below back to the implementer"))
    lines.append("")
    lines.append("TODO (where the difference sits, left -> right):")
    shown = 0
    for b in range(bands):
        if band_hits[b] == 0:
            continue
        lines.append("  band %d/%d  x[%4d..%4d)  %6d px  %.3f%% of band   <-- fix here"
                     % (b + 1, bands, b * width // bands, (b + 1) * width // bands,
                        band_hits[b], band_hits[b] / float(max(1, band_pixels[b])) * 100.0))
        shown += 1
    if shown == 0:
        lines.append("  (none)")
    text = "\n".join(lines)
    with open(os.path.join(args.out_dir, "delta.txt"), "w", encoding="utf-8") as fh:
        fh.write(text + "\n")

    if args.json_path:
        payload = {
            "baseline": args.baseline,
            "ours": args.ours,
            "normalised": {"width": width, "height": height},
            "diff_pixels": diff_count,
            "total_pixels": int(total),
            "diff_ratio": ratio,
            "ssim_mean": ssim_mean,
            "ssim_min_block": ssim_min,
            "aa_suppressed_pixels": aa_suppressed,
            "threshold": args.threshold,
            "max_diff_ratio": args.max_diff_ratio,
            "max_diff_pixels": args.max_diff_pixels,
            "min_ssim": args.min_ssim,
            "bands": bands,
            "band_hits": band_hits,
            "band_pixels": band_pixels,
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
