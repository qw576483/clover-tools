#!/usr/bin/env python3
"""ui-bbox -- measure the bounding box of a UI control on a screenshot.

Why this file exists (skill `clover-engine`, 判定权三分: "可计算 => 脚本"):
  "the button is 160x48 at (240,120)" is a CLAIM, and a claim needs a measurement.
  This is the half-generic measuring stick behind that claim. Give it a screenshot
  plus either a known control colour or ink mode, and it returns the box the pixels
  actually occupy (x/y/w/h/centre/fill), optionally checked against the box the
  layout was supposed to produce -- so a layout regression becomes a number with a
  region, not "the button looks a bit off".

  It complements the other two half-generic tools in this directory:
  shot-stats.py (is a colour present, and in which columns) / font-metrics.py
  (what the FONT FILE says about a glyph) -- this one answers "what box do the
  pixels of THIS control occupy".

Modes (--mode):
  color   measure the box of every --target NAME=#RRGGBB[:TOL[:Y0:Y1]] probe
          (same probe syntax as shot-stats.py --color, including the y-band).
  ink     measure the box of the ink inside --crop x,y,w,h: pixels whose RGB
          differs from --bg (default: the crop's modal colour) by more than
          --bg-tolerance. Use it for a control painted with a gradient, an icon
          or text -- anything without one flat colour.

Judgement (optional, and this is the part that makes it a criterion):
  --expect NAME=x,y,w,h[:TOL]  compare the measured box against the expected one;
  beyond the tolerance the target is reported MISMATCH, and a target with no
  pixels at all is reported MISSING. Both make the process exit 1. Without
  --expect the tool only reports (exit 0).

Nothing here is game specific: colours, crops and expectations all come from the
CLI. Any project with UI screenshots can reuse it.

Usage:
  python ui-bbox.py <shot.png> --mode color --target "ok=#1c8414:40"
  python ui-bbox.py <shot.png> --mode color --target "ok=#1c8414:40" --expect "ok=240,120,160,48:2"
  python ui-bbox.py <shot.png> --mode ink --crop 800,600,400,200 --name panel

Options:
  --mode color|ink        what to measure (default color)
  --target SPEC           colour probe NAME=#RRGGBB[:TOL[:Y0:Y1]] (repeatable, colour mode)
  --crop x,y,w,h          crop to measure (ink mode; default: the whole image)
  --bg #RRGGBB            ink-mode background (default: the crop's modal colour)
  --bg-tolerance N        sum |dR|+|dG|+|dB| still counted as background (default 30)
  --name NAME             ink-mode target name (default "ink")
  --expect SPEC           NAME=x,y,w,h[:TOL] (repeatable)
  --expect-tolerance N    default tolerance for --expect without `:TOL` (default 1)
  --out PATH              write the report here (default: stdout)
  --boxed PATH            write the shot with the measured boxes painted here
  --json PATH             also write the machine-readable result here

Exit codes: 0 = reported (every --expect met), 1 = a target was MISSING or its
box MISMATCHed, 2 = usage error, 3 = dependency error.
"""

import argparse
import json
import os
import sys


def die(code, msg):
    sys.stderr.write("ui-bbox: " + msg + "\n")
    raise SystemExit(code)


try:
    import numpy as np
    from PIL import Image
except ImportError as exc:  # pragma: no cover - depends on the host
    die(3, "missing dependency (%s) -- install with: python -m pip install -r requirements.txt" % exc)


# --------------------------------------------------------------------------- #
# parsing
# --------------------------------------------------------------------------- #
def parse_hex(text):
    t = str(text).strip().lstrip("#")
    if len(t) != 6:
        die(2, "colour must be #RRGGBB, got: " + str(text))
    try:
        return np.array([int(t[0:2], 16), int(t[2:4], 16), int(t[4:6], 16)], dtype=np.int32)
    except ValueError:
        die(2, "colour must be #RRGGBB, got: " + str(text))


def parse_color_target(spec):
    if "=" not in spec:
        die(2, "colour target must look like NAME=#RRGGBB[:TOL[:Y0:Y1]], got: " + spec)
    name, rest = spec.split("=", 1)
    parts = rest.split(":")
    rgb = parse_hex(parts[0])
    tol = int(parts[1]) if len(parts) >= 2 and parts[1] != "" else 40
    y0 = float(parts[2]) if len(parts) >= 3 and parts[2] != "" else 0.0
    y1 = float(parts[3]) if len(parts) >= 4 and parts[3] != "" else 1.0
    if y1 <= y0:
        die(2, "colour target %s: Y1 must be > Y0" % name)
    return {"name": name.strip(), "rgb": rgb, "tol": tol, "y0": y0, "y1": y1}


def parse_expect(spec):
    if "=" not in spec:
        die(2, "expectation must look like NAME=x,y,w,h[:TOL], got: " + spec)
    name, rest = spec.split("=", 1)
    body, _, tol_text = rest.partition(":")
    nums = body.split(",")
    if len(nums) != 4:
        die(2, "expectation %s must have 4 numbers x,y,w,h, got: %s" % (name, body))
    try:
        x, y, w, h = [int(n) for n in nums]
    except ValueError:
        die(2, "expectation %s must have integer x,y,w,h, got: %s" % (name, body))
    return {"name": name.strip(), "box": [x, y, w, h],
            "tol": int(tol_text) if tol_text != "" else None}


def parse_crop(text):
    nums = str(text).split(",")
    if len(nums) != 4:
        die(2, "--crop must be x,y,w,h, got: " + str(text))
    try:
        x, y, w, h = [int(n) for n in nums]
    except ValueError:
        die(2, "--crop must be integers x,y,w,h, got: " + str(text))
    if w <= 0 or h <= 0:
        die(2, "--crop w/h must be > 0")
    return x, y, w, h


# --------------------------------------------------------------------------- #
# measurement
# --------------------------------------------------------------------------- #
def hex_of(rgb):
    return "#%02x%02x%02x" % (int(rgb[0]), int(rgb[1]), int(rgb[2]))


def modal_color(a):
    flat = a.reshape(-1, 3)
    key = (flat[:, 0].astype(np.int64) << 16) | (flat[:, 1].astype(np.int64) << 8) | flat[:, 2]
    vals, cnt = np.unique(key, return_counts=True)
    v = int(vals[int(np.argmax(cnt))])
    return np.array([(v >> 16) & 255, (v >> 8) & 255, v & 255], dtype=np.int32)


def measure(mask, ox, oy):
    """mask is a HW bool array whose (0,0) sits at image coords (ox, oy)."""
    ys, xs = np.where(mask)
    pixels = int(len(xs))
    if pixels == 0:
        return {"pixels": 0, "bbox": None, "width": 0, "height": 0,
                "center": None, "fill_ratio": 0.0}
    x0, x1 = int(xs.min()) + ox, int(xs.max()) + ox
    y0, y1 = int(ys.min()) + oy, int(ys.max()) + oy
    w, h = x1 - x0 + 1, y1 - y0 + 1
    return {"pixels": pixels, "bbox": [x0, y0, x1, y1], "width": w, "height": h,
            "center": [(x0 + x1) / 2.0, (y0 + y1) / 2.0],
            "fill_ratio": pixels / float(max(1, w * h))}


def paint_rect(img, x0, y0, x1, y1, color, thickness=2):
    h, w = img.shape[:2]
    x0, y0 = max(0, x0), max(0, y0)
    x1, y1 = min(w - 1, x1), min(h - 1, y1)
    if x1 < x0 or y1 < y0:
        return
    img[y0:min(h, y0 + thickness), x0:x1 + 1] = color
    img[max(0, y1 - thickness + 1):y1 + 1, x0:x1 + 1] = color
    img[y0:y1 + 1, x0:min(w, x0 + thickness)] = color
    img[y0:y1 + 1, max(0, x1 - thickness + 1):x1 + 1] = color


# --------------------------------------------------------------------------- #
# main
# --------------------------------------------------------------------------- #
def build_parser():
    p = argparse.ArgumentParser(
        prog="ui-bbox.py",
        description="Measure a UI control's bounding box (colour probe or ink inside a crop), "
                    "optionally checked against the expected box.",
    )
    p.add_argument("shot", help="the screenshot (.png)")
    p.add_argument("--mode", choices=("color", "ink"), default="color", help="what to measure (default color)")
    p.add_argument("--target", action="append", default=[],
                   help="colour probe NAME=#RRGGBB[:TOL[:Y0:Y1]] (repeatable, colour mode)")
    p.add_argument("--crop", default="", help="ink mode: x,y,w,h (default: the whole image)")
    p.add_argument("--bg", default="", help="ink mode: background colour #RRGGBB (default: modal colour of the crop)")
    p.add_argument("--bg-tolerance", type=int, default=30,
                   help="sum |dR|+|dG|+|dB| still counted as background (default 30)")
    p.add_argument("--name", default="ink", help='ink mode target name (default "ink")')
    p.add_argument("--expect", action="append", default=[],
                   help="NAME=x,y,w,h[:TOL] (repeatable)")
    p.add_argument("--expect-tolerance", type=int, default=1,
                   help="default tolerance for --expect without :TOL (default 1)")
    p.add_argument("--out", default="", help="write the report here (default: stdout)")
    p.add_argument("--boxed", default="", help="write the shot with the measured boxes painted here")
    p.add_argument("--json", dest="json_path", default="", help="also write the machine-readable result here")
    return p


def main(argv=None):
    args = build_parser().parse_args(argv)
    if not os.path.isfile(args.shot):
        die(2, "no such file: " + args.shot)
    if args.bg_tolerance < 0:
        die(2, "--bg-tolerance must be >= 0")
    if args.mode == "color" and not args.target:
        die(2, "--mode color needs at least one --target")
    if args.mode == "ink" and args.target:
        die(2, "--target is a colour-mode option; ink mode uses --crop/--bg/--name")

    img = Image.open(args.shot).convert("RGB")
    a = np.asarray(img, dtype=np.int16)
    h, w = a.shape[:2]

    expects = {}
    for spec in args.expect:
        e = parse_expect(spec)
        if e["name"] in expects:
            die(2, "duplicate --expect for target: " + e["name"])
        expects[e["name"]] = e
    if args.expect and args.mode == "ink" and args.name not in expects:
        die(2, "ink mode: --expect must name '%s', got: %s" % (args.name, ", ".join(sorted(expects))))

    results = []
    colors = {}

    if args.mode == "color":
        targets = [parse_color_target(s) for s in args.target]
        for t in targets:
            y0 = max(0, int(round(h * t["y0"])))
            y1 = min(h, int(round(h * t["y1"])))
            body = a[y0:y1, :]
            if body.size == 0:
                die(2, "colour target %s: empty y-band" % t["name"])
            diff = np.abs(body.astype(np.int32) - t["rgb"].reshape(1, 1, 3)).sum(axis=2)
            m = measure(diff <= t["tol"], 0, y0)
            results.append({"name": t["name"], "mode": "color", "color": hex_of(t["rgb"]),
                            "tol": t["tol"], "y_band": [t["y0"], t["y1"]], **m})
            colors[t["name"]] = t["rgb"]
        for name in expects:
            if name not in colors:
                die(2, "no --target named '%s' for its --expect" % name)
    else:
        if args.crop:
            cx, cy, cw, ch = parse_crop(args.crop)
            if cx < 0 or cy < 0 or cx + cw > w or cy + ch > h:
                die(2, "--crop %dx%d+%d+%d is outside the %dx%d image" % (cw, ch, cx, cy, w, h))
        else:
            cx, cy, cw, ch = 0, 0, w, h
        body = a[cy:cy + ch, cx:cx + cw]
        bg = parse_hex(args.bg) if args.bg else modal_color(body)
        diff = np.abs(body.astype(np.int32) - bg.reshape(1, 1, 3)).sum(axis=2)
        m = measure(diff > args.bg_tolerance, cx, cy)
        results.append({"name": args.name, "mode": "ink", "bg": hex_of(bg),
                        "bg_tolerance": args.bg_tolerance, "crop": [cx, cy, cw, ch], **m})

    # ---- verdicts ----
    for r in results:
        e = expects.get(r["name"])
        if e is None:
            r["verdict"] = "MEASURED"
            r["expect"] = None
            continue
        r["expect"] = {"box": e["box"], "tol": e["tol"]}
        if r["pixels"] == 0 or r["bbox"] is None:
            r["verdict"] = "MISSING"
            continue
        ex, ey, ew, eh = e["box"]
        tol = args.expect_tolerance if e["tol"] is None else e["tol"]
        dx = r["bbox"][0] - ex
        dy = r["bbox"][1] - ey
        dw = r["width"] - ew
        dh = r["height"] - eh
        r["delta"] = [int(dx), int(dy), int(dw), int(dh)]
        r["tol"] = tol
        r["verdict"] = "OK" if max(abs(dx), abs(dy), abs(dw), abs(dh)) <= tol else "MISMATCH"

    bad = [r for r in results if r["verdict"] in ("MISSING", "MISMATCH")]
    lines = []
    lines.append("# ui-bbox: %s  %dx%d  mode=%s" % (os.path.basename(args.shot), w, h, args.mode))
    lines.append("# caliber: expect_tolerance=%d  (bg_tolerance=%d)" % (args.expect_tolerance, args.bg_tolerance))
    for r in results:
        head = "target %-14s mode=%-5s" % (r["name"], r["mode"])
        if r["mode"] == "color":
            head += " %s tol=%d y=[%.2f..%.2f]" % (r["color"], r["tol"], r["y_band"][0], r["y_band"][1])
        else:
            head += " bg=%s crop=%s" % (r["bg"], ",".join(str(v) for v in r["crop"]))
        lines.append(head)
        if r["pixels"] == 0:
            lines.append("  pixels=0  bbox=-  verdict=%s (nothing matched)" % r["verdict"])
            continue
        lines.append("  pixels=%-8d bbox=%d,%d-%d,%d  w=%d h=%d  center=(%.1f,%.1f)  fill=%.6f"
                     % (r["pixels"], r["bbox"][0], r["bbox"][1], r["bbox"][2], r["bbox"][3],
                        r["width"], r["height"], r["center"][0], r["center"][1], r["fill_ratio"]))
        if r["expect"] is not None:
            lines.append("  expect=%s tol=%d  delta(dx,dy,dw,dh)=%s  verdict=%s"
                         % (",".join(str(v) for v in r["expect"]["box"]), r["tol"],
                            r.get("delta", "-"), r["verdict"]))
        else:
            lines.append("  verdict=%s (no --expect: reported only)" % r["verdict"])
    lines.append("summary  targets=%d missing=%d mismatch=%d  result=%s"
                 % (len(results),
                    len([r for r in results if r["verdict"] == "MISSING"]),
                    len([r for r in results if r["verdict"] == "MISMATCH"]),
                    "FAIL" if bad else "PASS"))
    text = "\n".join(lines) + "\n"

    if args.boxed:
        out_dir = os.path.dirname(os.path.abspath(args.boxed))
        if out_dir:
            os.makedirs(out_dir, exist_ok=True)
        canvas = np.array(img, dtype=np.uint8)
        for r in results:
            if r["verdict"] in ("MISSING", "MISMATCH"):
                color = (255, 0, 0)
            else:
                color = (0, 255, 0)
            if r["bbox"]:
                paint_rect(canvas, r["bbox"][0], r["bbox"][1], r["bbox"][2], r["bbox"][3], color)
            if r["expect"] is not None:
                ex, ey, ew, eh = r["expect"]["box"]
                paint_rect(canvas, ex, ey, ex + ew - 1, ey + eh - 1, (255, 220, 0))
        Image.fromarray(canvas).save(args.boxed)

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
            "shot": args.shot, "width": int(w), "height": int(h), "mode": args.mode,
            "expect_tolerance": args.expect_tolerance, "bg_tolerance": args.bg_tolerance,
            "targets": results,
            "missing": [r["name"] for r in results if r["verdict"] == "MISSING"],
            "mismatch": [r["name"] for r in results if r["verdict"] == "MISMATCH"],
            "result": "FAIL" if bad else "PASS",
        }
        with open(args.json_path, "w", encoding="utf-8") as fh:
            json.dump(payload, fh, ensure_ascii=False, indent=2, sort_keys=True)
            fh.write("\n")

    return 1 if bad else 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except BrokenPipeError:  # pragma: no cover
        sys.exit(0)
