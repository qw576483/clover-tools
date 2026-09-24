#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""compose-contact-sheet -- N 张截图 -> 1 张联络图 + 一份 *-index.tsv。

为什么必须有它（skill clover-engine -> reference/visual-loop.md 第八节）：
  AI 读图**按像素烧 token**。判定权在"只有眼睛能判"的那一类（`表现类`）时，
  读图的人只读**一张汇总图**，不逐张开图；而"哪一格对应哪张取证图、格上那串数字
  是从哪来的"必须能**查回来**（否则一张拼图可以看起来"都验过了"）。
  => 每格左上角烧**格号**，格下烧**状态名 + 类别 + 关键数值**，
     同时输出 `index.tsv`：`格号 <-> 截图来源 <-> 状态 <-> 类别 <-> 关键数值`。

与 `capture-editor-window.ps1` 的关系：后者负责**采**（窗口级像素，用户真正看到的画面），
本脚本负责**拼**（把 N 个证据点压进 1 张图 + 索引表）。两者都是判据资产，都不改被验对象。

确定性（硬要求，可复核）：
  * 输入顺序 = 命令行给出的顺序；单个目录 / glob 内部用 `sorted()` 定序（同输入同顺序）；
  * 每格缩放到统一框内再**整数居中**粘贴（坐标 / 尺寸全由 `--cell-size` 与 `--cols` 算出）；
  * 缩略图固定用 `LANCZOS`；
  * **本工具不自动烧时间戳**（时间会破坏"两次跑同 SHA256"）；构建标识 / 采集时间
    **由调用方通过 `--title` 传入**（要复算时传同一个串）。
  * 输出 PNG / TSV 均不含可变元数据 => 同输入两次跑，两个文件 SHA256 相同。

用法：
  python compose-contact-sheet.py --shots ".ai-tmp/screenshots/a.png,b.png" \
      --labels "01=菜单|表现类|标题 y=64;02=创角|表现类|3 个输入框" \
      --cols 2 --out .ai-tmp/test/sheet.png --index .ai-tmp/test/sheet.index.tsv

  # --shots 支持：目录（取其中 *.png，排序）/ glob / 单个文件；逗号或分号分隔，可重复。
  # --labels 支持：分号分隔的 `[格号=]状态名[|类别|关键数值]`，
  #                给 `格号=` 时按格号绑定，不给时按 --shots 顺序绑定（数量必须一致）。

退出码：0 = 成功；2 = 用法错（参数非法 / 标签数量不匹配）；3 = 依赖缺失（Pillow）；
        4 = 输入缺失或不可读（**缺图即失败，绝不半发布** —— 校验全部通过后才写盘）。
"""

import argparse
import glob
import hashlib
import os
import sys

try:
    from PIL import Image, ImageDraw, ImageFont
except ImportError as exc:  # pragma: no cover - depends on the host
    sys.stderr.write(
        "compose-contact-sheet: Pillow missing (%s)\n"
        "  install: python -m pip install -r clover-tools/visual-verify/requirements.txt\n" % exc
    )
    raise SystemExit(3)

# ── stdout / stderr 编码（实测本机 python 3.12 + 重定向：sys.stdout.encoding == "gbk"）──────
#  为什么必须显式改：① 目标路径里若有 **非 GBK 字符**（如 ⛔ / 部分 emoji），`print` 会抛
#  UnicodeEncodeError ⇒ **文件其实已经写好**，进程却以 exit 1 收场 —— 典型的"假红 + 状态
#  与判据不一致"；② GBK 可编码的中文会被写成 **GBK 字节**，调用方（AI 宿主 / 日志按 UTF-8
#  读）拿到的就是 `??` 乱码 ⇒ **证据行读不回来**。
#  改法：把两个流显式设为 UTF-8，并把不可编码字符转成 `\uXXXX`（errors=backslashreplace：
#  不丢信息、也不崩）。⛔ 本工具不改任何**被验对象**的编码。
for _stream in (sys.stdout, sys.stderr):
    try:
        _stream.reconfigure(encoding="utf-8", errors="backslashreplace")
    except Exception:
        pass

USAGE_ERR = 2
INPUT_ERR = 4

# 仓库根（本文件 = <仓库根>/clover-tools/visual-verify/compose-contact-sheet.py）。
# 只作为**相对路径的第二顺位**解析（第一顺位是当前工作目录），不写死任何机器路径。
ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

# ============================================================================
#  CJK 字体（⛔ 只从系统字体里取，⛔ 不下载）
# ----------------------------------------------------------------------------
#  为什么必须有它：PIL 的默认位图字体（ImageFont.load_default()）只含 ASCII ⇒ 图内**中文
#  渲染成方块**。判定权在"只有眼睛能判"那一类时，读图的人只读这张汇总图 ⇒ 中文标签必须
#  是字，不能是方块。取第一个存在的：微软雅黑 → 黑体 → 宋体；都没有才退回默认字体
#  （会成方块，但不崩），并在 stderr 留一行告警。
# ============================================================================
CJK_FONT_CANDIDATES = (
    r"C:\Windows\Fonts\msyh.ttc",    # 微软雅黑（Win7+）
    r"C:\Windows\Fonts\simhei.ttf",  # 黑体
    r"C:\Windows\Fonts\simsun.ttc",  # 宋体
)


def load_font(size):
    for path in CJK_FONT_CANDIDATES:
        if os.path.exists(path):
            try:
                return ImageFont.truetype(path, size)
            except Exception:
                continue
    sys.stderr.write(
        "compose-contact-sheet: WARN no CJK font found (tried %s); "
        "labels will render as boxes\n" % ", ".join(CJK_FONT_CANDIDATES)
    )
    return ImageFont.load_default()


def fit_text(draw, text, font, max_w):
    """把标签裁到 max_w 内（用 … 收尾）—— 只影响文字，⛔ 不改图尺寸。"""
    if draw.textlength(text, font=font) <= max_w:
        return text
    while text and draw.textlength(text + "\u2026", font=font) > max_w:
        text = text[:-1]
    return text + "\u2026"


def resolve_path(p):
    """绝对路径原样；相对路径先按 CWD，再按仓库根（ROOT）——两处都不在则返回 CWD 解析值。"""
    if os.path.isabs(p):
        return os.path.normpath(p)
    cwd_cand = os.path.abspath(p)
    if os.path.exists(cwd_cand):
        return cwd_cand
    root_cand = os.path.normpath(os.path.join(ROOT, p.replace("/", os.sep)))
    if os.path.exists(root_cand):
        return root_cand
    return cwd_cand


def expand_shots(tokens):
    """把 --shots 的 token 展开成**有序**文件列表。

    单个 token 的三种形态：目录（取其中 *.png，sorted）/ glob（sorted）/ 单个文件。
    一个 token 命中 0 个文件 = 输入缺失（⛔ 不静默跳过）。
    """
    out = []
    for tok in tokens:
        for part in [t.strip() for t in tok.replace("\n", ",").split(",")]:
            for sub in part.split(";"):
                sub = sub.strip()
                if not sub:
                    continue
                raw = sub if os.path.isabs(sub) else sub
                if os.path.isdir(resolve_path(raw)):
                    d = resolve_path(raw)
                    hits = sorted(
                        os.path.join(d, n)
                        for n in os.listdir(d)
                        if n.lower().endswith(".png")
                    )
                elif any(ch in sub for ch in "*?["):
                    hits = sorted(resolve_path(g) for g in glob.glob(sub))
                    if not hits:
                        hits = sorted(glob.glob(os.path.join(ROOT, sub)))
                    hits = [h for h in hits if os.path.isfile(h)]
                else:
                    hits = [resolve_path(sub)]
                if not hits:
                    sys.stderr.write(
                        "compose-contact-sheet: --shots entry matched no file: %s\n" % sub
                    )
                    raise SystemExit(INPUT_ERR)
                out.extend(hits)
    if not out:
        sys.stderr.write("compose-contact-sheet: --shots must resolve to at least one image\n")
        raise SystemExit(USAGE_ERR)
    return out


def parse_labels(tokens, n_cells):
    """解析 --labels -> {格号: (状态, 类别, 关键数值)}。

    条目 = `[格号=]状态名[|类别|关键数值]`，分号分隔。
    给了 `格号=` 就按格号绑定；没给就按 --shots 顺序绑定（数量必须与格数一致）。
    带 `|` 时必须三段齐全（状态名|类别|关键数值）—— 位置含糊比报错更贵。
    """
    entries = []
    for tok in tokens:
        for entry in tok.replace("\n", ";").split(";"):
            entry = entry.strip()
            if entry:
                entries.append(entry)
    if not entries:
        return {("%02d" % (i + 1)): ("", "", "") for i in range(n_cells)}

    positional = [e for e in entries if "=" not in e.split("|")[0]]
    keyed = [e for e in entries if "=" in e.split("|")[0]]
    if positional and len(positional) != n_cells:
        sys.stderr.write(
            "compose-contact-sheet: %d positional labels for %d cells "
            "(use NN=... to bind by cell number)\n" % (len(positional), n_cells)
        )
        raise SystemExit(USAGE_ERR)

    result = {}
    for i, e in enumerate(positional):
        result["%02d" % (i + 1)] = split_label(e)
    for e in keyed:
        num, _, rest = e.partition("=")
        num = num.strip()
        if len(num) == 1:
            num = "0" + num  # 与格号口径统一（01..09 / 10..）
        result[num] = split_label(rest.strip())
    return result


def split_label(text):
    parts = [p.strip() for p in text.split("|")]
    if len(parts) == 1:
        return (parts[0], "", "")
    if len(parts) == 3:
        return (parts[0], parts[1], parts[2])
    sys.stderr.write(
        "compose-contact-sheet: bad label %r (want `状态名` or `状态名|类别|关键数值`)\n" % text
    )
    raise SystemExit(USAGE_ERR)


def sha256_file(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 16), b""):
            h.update(chunk)
    return h.hexdigest()


def main(argv=None):
    ap = argparse.ArgumentParser(
        prog="compose-contact-sheet.py",
        description="把 N 张截图拼成一张联络图，并输出 *-index.tsv 索引表。",
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    ap.add_argument("--shots", action="append", required=True, metavar="SPEC",
                    help="截图来源：目录 / glob / 单个文件；逗号或分号分隔，可重复。")
    ap.add_argument("--cols", type=int, default=4, metavar="N", help="列数（默认 4）。")
    ap.add_argument("--labels", action="append", default=[], metavar="SPEC",
                    help="分号分隔的 `[格号=]状态名[|类别|关键数值]`；可重复。")
    ap.add_argument("--out", required=True, metavar="PNG", help="联络图输出路径（必填）。")
    ap.add_argument("--index", metavar="TSV", default=None,
                    help="索引表输出路径（默认 <out 去扩展名>.index.tsv）。")
    ap.add_argument("--title", default="", metavar="T",
                    help="顶部标题（构建标识 / 采集时间等由调用方传入）。")
    ap.add_argument("--cell-size", default="480x320", metavar="WxH",
                    help="每格图像框尺寸（默认 480x320）。")
    args = ap.parse_args(argv)

    if args.cols < 1:
        sys.stderr.write("compose-contact-sheet: --cols must be >= 1\n")
        return USAGE_ERR
    try:
        cell_w, cell_h = (int(x) for x in args.cell_size.lower().split("x"))
    except Exception:
        sys.stderr.write("compose-contact-sheet: --cell-size wants WxH (e.g. 480x320)\n")
        return USAGE_ERR
    if cell_w <= 0 or cell_h <= 0:
        sys.stderr.write("compose-contact-sheet: --cell-size must be positive\n")
        return USAGE_ERR

    shots = expand_shots(args.shots)
    index_path = args.index or os.path.splitext(args.out)[0] + ".index.tsv"

    # ── 把**本工具自己的产物**从输入里剔掉 ────────────────────────────────
    #  实测（2026-09-24）：`--shots <目录>` 且 `--out` 也落在这个目录时，第二遍跑会把
    #  第一遍写出的 `sheet.png` 当成输入 ⇒ 格数 7 -> 8、SHA 全变 —— "同输入两次跑同 SHA"
    #  这条确定性承诺**当场失效**，而且看起来像工具不确定（其实是输入变了）。
    #  ⇒ 输出文件永不算输入；剔掉时在 stderr 留一行（⛔ 不静默改输入集）。
    _excl = set()
    for _p in (args.out, index_path):
        if _p:
            _excl.add(os.path.normcase(os.path.abspath(_p)))
    _kept = [s for s in shots if os.path.normcase(os.path.abspath(s)) not in _excl]
    if len(_kept) != len(shots):
        sys.stderr.write(
            "compose-contact-sheet: excluded %d of its own output file(s) from --shots "
            "(out/index are never inputs)\n" % (len(shots) - len(_kept))
        )
        shots = _kept
    labels = parse_labels(args.labels, len(shots))

    # ── 先全量校验输入（缺图 = 失败，且**一个字节都不写**）──────────────────
    missing = [s for s in shots if not os.path.isfile(s)]
    if missing:
        for s in missing:
            sys.stderr.write("compose-contact-sheet: screenshot missing: %s\n" % s)
        sys.stderr.write("compose-contact-sheet: refusing to publish a partial sheet\n")
        return INPUT_ERR

    # 格号 = 01..N（与 --shots 顺序一一对应，也就是 index.tsv 与图上的同一口径）。
    # 键式标签（`NN=...`）只能落在已有格号上：多出来的格号 = 打错，直接报错。
    nums = ["%02d" % (i + 1) for i in range(len(shots))]
    for n in sorted(labels):
        if n not in nums:
            sys.stderr.write(
                "compose-contact-sheet: label cell %s has no matching cell (1..%d)\n"
                % (n, len(shots))
            )
            return USAGE_ERR

    # ── 拼版 ──────────────────────────────────────────────────────────────
    rows = (len(shots) + args.cols - 1) // args.cols
    label_h = 44
    head_h = 30 if args.title else 0
    W = args.cols * cell_w
    H = head_h + rows * (cell_h + label_h)
    sheet = Image.new("RGB", (W, H), (20, 20, 24))
    d = ImageDraw.Draw(sheet)
    title_font = load_font(16)
    label_font = load_font(14)
    num_font = load_font(13)
    if args.title:
        d.text((6, 6), fit_text(d, args.title, title_font, W - 12),
               fill=(235, 235, 240), font=title_font)

    index_rows = []
    for k, path in enumerate(shots):
        num = nums[k]
        state, category, values = labels.get(num, ("", "", ""))
        r, c = divmod(k, args.cols)
        x = c * cell_w
        y = head_h + r * (cell_h + label_h)
        im = Image.open(path).convert("RGB")
        im.thumbnail((cell_w, cell_h), Image.LANCZOS)
        sheet.paste(im, (x + (cell_w - im.width) // 2, y + (cell_h - im.height) // 2))
        d.rectangle([x, y, x + cell_w - 1, y + cell_h - 1], outline=(70, 70, 80))
        # 格号画在左上角（白底黑字，保证在任意画面上都读得出来）
        nw = 8 + 7 * len(num)
        d.rectangle([x + 2, y + 2, x + 2 + nw, y + 18], fill=(255, 255, 255))
        d.text((x + 6, y + 3), num, fill=(0, 0, 0), font=num_font)
        # 格下两行：① 状态名 ② [类别] 关键数值
        line1 = (num + "  " + state).rstrip()
        line2 = (("[%s] " % category) if category else "") + values
        d.text((x + 4, y + cell_h + 5), fit_text(d, line1, label_font, cell_w - 8),
               fill=(210, 210, 220), font=label_font)
        if line2:
            d.text((x + 4, y + cell_h + 24), fit_text(d, line2, label_font, cell_w - 8),
                   fill=(225, 200, 140), font=label_font)
        index_rows.append((num, os.path.relpath(path, ROOT).replace(os.sep, "/"),
                           state, category, values))

    # ── 校验全部通过后才写盘（不半发布）──────────────────────────────────
    out_dir = os.path.dirname(os.path.abspath(args.out))
    if out_dir and not os.path.isdir(out_dir):
        os.makedirs(out_dir, exist_ok=True)
    sheet.save(args.out, "PNG", optimize=False)
    idx_dir = os.path.dirname(os.path.abspath(index_path))
    if idx_dir and not os.path.isdir(idx_dir):
        os.makedirs(idx_dir, exist_ok=True)
    with open(index_path, "w", encoding="utf-8", newline="\n") as f:
        f.write("格号\t截图来源\t状态\t类别\t关键数值\n")
        for row in index_rows:
            f.write("\t".join(row) + "\n")

    print("contact sheet -> %s  (%dx%d, %d cells)" % (args.out, W, H, len(shots)))
    print("index         -> %s" % index_path)
    print("sha256 out    = %s" % sha256_file(args.out))
    print("sha256 index  = %s" % sha256_file(index_path))
    return 0


if __name__ == "__main__":
    sys.exit(main())
