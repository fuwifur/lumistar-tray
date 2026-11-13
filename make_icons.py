"""Builds setup.ico (and Lumistar.ico): the Lumistar star on the dark-theme panel colour #16121f.
Rounded-square tile, transparent outside the corners, written as standard 32-bit icon images with a proper AND mask."""
import math, struct
from PIL import Image, ImageDraw, ImageFilter

S = 1024
cx, cy, R, r = S/2, S/2 + S*0.025, S*0.36, S*0.165
pts = []
for i in range(10):
    a = -math.pi/2 + i*math.pi/5
    pts.append((cx + (R if i % 2 == 0 else r)*math.cos(a), cy + (R if i % 2 == 0 else r)*math.sin(a)))
m = Image.new("L", (S, S), 0)
ImageDraw.Draw(m).polygon(pts, fill=255)
m = m.filter(ImageFilter.GaussianBlur(S*0.022)).point(lambda v: 255 if v > 128 else 0).filter(ImageFilter.GaussianBlur(1))
c1, c2 = (167, 139, 250), (91, 33, 182)
g = Image.new("RGB", (S, S)); px = g.load()
for y in range(S):
    for x in range(S):
        t = (x + y) / (2*S); px[x, y] = tuple(int(c1[i] + (c2[i]-c1[i])*t) for i in range(3))
star = g.convert("RGBA"); star.putalpha(m); star = star.crop(m.getbbox())
side = max(star.size); sq = Image.new("RGBA", (side, side), (0, 0, 0, 0))
sq.paste(star, ((side-star.width)//2, (side-star.height)//2))

SS = 4  # supersampling factor for smooth rounded edges

def tile(n):
    """Dark rounded square (same corner softness as the Claude icon) with the star centred on it.
    Corners outside the rounded square are transparent; everything inside is fully opaque."""
    big = n * SS
    radius = round(big * 0.225)            # soft corners, like the Claude icon
    base = Image.new("RGBA", (big, big), (0, 0, 0, 0))
    ImageDraw.Draw(base).rounded_rectangle((0, 0, big-1, big-1), radius=radius, fill=(22, 18, 31, 255))
    margin = round(big * 0.16)              # same star margin on all four sides
    k = big - 2*margin
    layer = Image.new("RGBA", (big, big), (0, 0, 0, 0))
    layer.paste(sq.resize((k, k), Image.LANCZOS), (margin, margin), sq.resize((k, k), Image.LANCZOS))
    out = Image.alpha_composite(base, layer)
    return out.resize((n, n), Image.LANCZOS)

def bmp_entry(img):
    n = img.width
    raw = img.tobytes("raw", "BGRA")
    rows = [raw[y*n*4:(y+1)*n*4] for y in range(n)]
    xor = b"".join(reversed(rows))
    # AND mask: bit 1 = transparent (alpha < 128), bit 0 = visible. Rows bottom-up, padded to 4 bytes.
    stride = ((n + 31) // 32) * 4
    mask = bytearray()
    for y in reversed(range(n)):
        row = bytearray(stride)
        for x in range(n):
            if img.getpixel((x, y))[3] < 128:
                row[x // 8] |= 0x80 >> (x % 8)
        mask += row
    return struct.pack("<IiiHHIIiiII", 40, n, n*2, 1, 32, 0, len(xor)+len(mask), 0, 0, 0, 0) + xor + bytes(mask)

sizes = [16, 24, 32, 48, 64, 128, 256]
entries = [bmp_entry(tile(n)) for n in sizes]
out = struct.pack("<HHH", 0, 1, len(entries)); offset = 6 + 16*len(entries); body = b""
for n, data in zip(sizes, entries):
    out += struct.pack("<BBBBHHII", n % 256, n % 256, 0, 0, 1, 32, len(data), offset + len(body)); body += data
open("setup.ico", "wb").write(out + body)
tile(512).save("icon_preview.png")
