from pathlib import Path
from PIL import Image, ImageDraw, ImageFilter


SIZE = 1024
OUTPUT = Path(__file__).resolve().parents[1] / "build" / "appicon.png"


def rounded_mask(size: int, radius: int) -> Image.Image:
    mask = Image.new("L", (size, size), 0)
    ImageDraw.Draw(mask).rounded_rectangle((0, 0, size - 1, size - 1), radius=radius, fill=255)
    return mask


def main() -> None:
    canvas = Image.new("RGBA", (SIZE, SIZE), (0, 0, 0, 0))
    shadow = Image.new("RGBA", (SIZE, SIZE), (0, 0, 0, 0))
    shadow_mask = rounded_mask(820, 188).filter(ImageFilter.GaussianBlur(38))
    shadow.paste((19, 24, 40, 115), (102, 120), shadow_mask)
    canvas.alpha_composite(shadow)

    tile = Image.new("RGBA", (820, 820), (0, 0, 0, 0))
    pixels = tile.load()
    for y in range(820):
        for x in range(820):
            diagonal = (x + y) / 1640
            glow = max(0.0, 1.0 - (((x - 630) ** 2 + (y - 190) ** 2) ** 0.5) / 620)
            r = int(24 + 31 * diagonal + 38 * glow)
            g = int(27 + 25 * diagonal + 22 * glow)
            b = int(34 + 38 * diagonal + 112 * glow)
            pixels[x, y] = (r, g, b, 255)
    tile.putalpha(rounded_mask(820, 188))
    canvas.alpha_composite(tile, (102, 92))

    shine = Image.new("RGBA", (820, 820), (0, 0, 0, 0))
    shine_draw = ImageDraw.Draw(shine)
    for radius in range(250, 0, -3):
        alpha = int(22 * (1 - radius / 250) ** 2)
        shine_draw.ellipse((565 - radius, 170 - radius, 565 + radius, 170 + radius), fill=(129, 119, 255, alpha))
    shine.putalpha(Image.composite(shine.getchannel("A"), Image.new("L", (820, 820), 0), rounded_mask(820, 188)))
    canvas.alpha_composite(shine, (102, 92))

    draw = ImageDraw.Draw(canvas)
    ink = (248, 250, 252, 255)
    soft = (163, 154, 255, 255)
    draw.rounded_rectangle((300, 286, 424, 738), radius=40, fill=ink)
    draw.rounded_rectangle((600, 286, 724, 738), radius=40, fill=ink)
    draw.rounded_rectangle((391, 444, 633, 568), radius=36, fill=ink)
    draw.rounded_rectangle((452, 225, 572, 349), radius=38, fill=soft)
    draw.rounded_rectangle((452, 675, 572, 799), radius=38, fill=soft)

    highlight = Image.new("RGBA", (820, 820), (255, 255, 255, 0))
    ImageDraw.Draw(highlight).rounded_rectangle((2, 2, 817, 817), radius=186, outline=(255, 255, 255, 38), width=5)
    canvas.alpha_composite(highlight, (102, 92))

    OUTPUT.parent.mkdir(parents=True, exist_ok=True)
    canvas.save(OUTPUT, "PNG", optimize=True)
    print(OUTPUT)


if __name__ == "__main__":
    main()
