#!/usr/bin/env python3
"""Generate Schlangen.TV app icons and favicon.

Draws a stylized snake head with body segments on the game's dark background,
matching the in-game neon aesthetic.
"""

import math
import os
from PIL import Image, ImageDraw, ImageFont, ImageFilter

# Game palette
BG = (10, 10, 46)          # #0a0a2e
SNAKE_HEAD = (255, 68, 102) # #ff4466 (first color in palette)
SNAKE_BODY = (204, 34, 68)  # #cc2244
EYE_WHITE = (255, 255, 255)
EYE_PUPIL = (20, 20, 30)
FOOD_COLORS = [
    (255, 107, 107), (238, 90, 36), (255, 211, 42),
    (11, 232, 129), (24, 220, 255), (113, 88, 226),
    (58, 227, 116), (255, 159, 67), (165, 94, 234),
]
GRID_COLOR = (255, 255, 255, 10)  # very subtle

GLOW_RADIUS = [0.0, 1.04, 1.08, 1.12]  # for body segments glow effect

def draw_icon(size, padding_frac=0.08):
    """Draw the snake icon at the given square size."""
    img = Image.new("RGBA", (size, size), BG + (255,))
    draw = ImageDraw.Draw(img)
    cx, cy = size / 2, size / 2
    pad = size * padding_frac
    usable = size - 2 * pad

    # Draw subtle grid
    grid_step = max(int(size / 10), 8)
    for x in range(0, size, grid_step):
        draw.line([(x, 0), (x, size)], fill=GRID_COLOR, width=1)
    for y in range(0, size, grid_step):
        draw.line([(0, y), (size, y)], fill=GRID_COLOR, width=1)

    # Scatter food particles in background
    import random
    random.seed(42)  # deterministic
    food_r = max(int(size * 0.012), 3)
    for _ in range(25):
        fx = random.randint(int(pad), int(size - pad))
        fy = random.randint(int(pad), int(size - pad))
        fc = FOOD_COLORS[random.randint(0, len(FOOD_COLORS) - 1)]
        # glow
        glow_r = food_r * GLOW_RADIUS[2]
        for gr in range(int(glow_r), food_r, -1):
            alpha = int(30 * (1 - (gr - food_r) / (glow_r - food_r)))
            draw.ellipse(
                [fx - gr, fy - gr, fx + gr, fy + gr],
                fill=fc + (alpha,)
            )
        draw.ellipse(
            [fx - food_r, fy - food_r, fx + food_r, fy + food_r],
            fill=fc + (220,)
        )

    # Snake body: curving S-shape from bottom-left to center
    head_r = usable * 0.09
    body_r = usable * 0.07
    seg_count = 16

    # Generate S-curve path
    path = []
    for i in range(seg_count + 6):  # extra segments for tail
        t = i / (seg_count + 5)
        # S curve: starts bottom-left, curves to upper-right, head at center-right
        px = pad + usable * (0.15 + t * 0.55)
        py = cy + usable * 0.28 * math.sin(t * math.pi * 2.2 - 0.5)
        path.append((px, py))

    # Draw body segments (tail to head)
    for i in range(len(path) - 1, 0, -1):
        px, py = path[i]
        t = 1 - (i / len(path))  # 0 at tail, 1 at head
        r = body_r * (0.4 + 0.6 * t)
        # Alternating stripes
        color = SNAKE_HEAD if (i // 2) % 2 == 0 else SNAKE_BODY
        alpha = int(180 + 75 * t)
        # Glow
        draw.ellipse(
            [px - r * GLOW_RADIUS[1], py - r * GLOW_RADIUS[1], px + r * GLOW_RADIUS[1], py + r * GLOW_RADIUS[1]],
            fill=color + (int(alpha * 0.15),)
        )
        draw.ellipse(
            [px - r, py - r, px + r, py + r],
            fill=color + (alpha,)
        )

    # Head
    hx, hy = path[0]
    # Direction from segment 1 to segment 0
    dx = path[0][0] - path[1][0]
    dy = path[0][1] - path[1][1]
    heading = math.atan2(dy, dx)

    # Head glow
    draw.ellipse(
        [hx - head_r * GLOW_RADIUS[2], hy - head_r * GLOW_RADIUS[2], hx + head_r * GLOW_RADIUS[2], hy + head_r * GLOW_RADIUS[2]],
        fill=SNAKE_HEAD + (40,)
    )
    # Head body
    draw.ellipse(
        [hx - head_r, hy - head_r, hx + head_r, hy + head_r],
        fill=SNAKE_HEAD + (255,)
    )
    # Head outline
    draw.ellipse(
        [hx - head_r, hy - head_r, hx + head_r, hy + head_r],
        outline=SNAKE_BODY + (255,), width=max(int(head_r * 0.15), 2)
    )

    # Eyes
    eye_dist = head_r * 0.5
    eye_r = head_r * 0.32
    pupil_r = eye_r * 0.55
    perp = heading + math.pi / 2
    for side in [-1, 1]:
        ex = hx + math.cos(heading) * head_r * 0.3 + math.cos(perp) * eye_dist * side
        ey = hy + math.sin(heading) * head_r * 0.3 + math.sin(perp) * eye_dist * side
        draw.ellipse(
            [ex - eye_r, ey - eye_r, ex + eye_r, ey + eye_r],
            fill=EYE_WHITE
        )
        # Pupil slightly forward
        ppx = ex + math.cos(heading) * eye_r * 0.25
        ppy = ey + math.sin(heading) * eye_r * 0.25
        draw.ellipse(
            [ppx - pupil_r, ppy - pupil_r, ppx + pupil_r, ppy + pupil_r],
            fill=EYE_PUPIL
        )

    return img


def draw_icon_rect(width, height, padding_frac=0.06, layer=None):
    """Draw the snake icon at a rectangular size (for tvOS top shelf etc).

    layer=None: full composite (default)
    layer="back": opaque background with grid only (RGB)
    layer="middle": food particles on transparent background (RGBA)
    layer="front": snake only on transparent background (RGBA)
    """
    if layer in ("front", "middle"):
        img = Image.new("RGBA", (width, height), (0, 0, 0, 0))
    else:
        img = Image.new("RGBA", (width, height), BG + (255,))
    draw = ImageDraw.Draw(img)
    cx, cy = width / 2, height / 2
    pad_x = width * padding_frac
    pad_y = height * padding_frac
    scale = min(width, height)

    # Draw subtle grid (back layer or full composite)
    if layer not in ("front", "middle"):
        grid_overlay = Image.new("RGBA", (width, height), (0, 0, 0, 0))
        grid_draw = ImageDraw.Draw(grid_overlay)
        grid_step = max(int(scale / 12), 8)
        for x in range(0, width, grid_step):
            grid_draw.line([(x, 0), (x, height)], fill=GRID_COLOR, width=1)
        for y in range(0, height, grid_step):
            grid_draw.line([(0, y), (width, y)], fill=GRID_COLOR, width=1)
        img = Image.alpha_composite(img, grid_overlay)
        draw = ImageDraw.Draw(img)

    # Back layer is just bg + grid
    if layer == "back":
        return img.convert("RGB")

    # Scatter food particles (middle layer, or full composite)
    if layer != "front":
        import random
        random.seed(42)
        food_r = max(int(scale * 0.012), 3)
        for _ in range(30):
            fx = random.randint(int(pad_x), int(width - pad_x))
            fy = random.randint(int(pad_y), int(height - pad_y))
            fc = FOOD_COLORS[random.randint(0, len(FOOD_COLORS) - 1)]
            glow_r = food_r * GLOW_RADIUS[2]
            for gr in range(int(glow_r), food_r, -1):
                alpha = int(30 * (1 - (gr - food_r) / (glow_r - food_r)))
                draw.ellipse(
                    [fx - gr, fy - gr, fx + gr, fy + gr],
                    fill=fc + (alpha,)
                )
            draw.ellipse(
                [fx - food_r, fy - food_r, fx + food_r, fy + food_r],
                fill=fc + (220,)
            )

    # Middle layer is just food particles
    if layer == "middle":
        return img

    # Snake body: S-curve across the width
    head_r = scale * 0.08
    body_r = scale * 0.06
    seg_count = 20

    path = []
    for i in range(seg_count + 8):
        t = i / (seg_count + 7)
        px = pad_x + (width - 2 * pad_x) * (0.1 + t * 0.6)
        py = cy + (height * 0.25) * math.sin(t * math.pi * 2.5 - 0.3)
        path.append((px, py))

    for i in range(len(path) - 1, 0, -1):
        px, py = path[i]
        t = 1 - (i / len(path))
        r = body_r * (0.35 + 0.65 * t)
        color = SNAKE_HEAD if (i // 2) % 2 == 0 else SNAKE_BODY
        alpha = int(180 + 75 * t)
        draw.ellipse(
            [px - r * GLOW_RADIUS[1], py - r * GLOW_RADIUS[1], px + r * GLOW_RADIUS[1], py + r * GLOW_RADIUS[1]],
            fill=color + (int(alpha * 0.15),)
        )
        draw.ellipse(
            [px - r, py - r, px + r, py + r],
            fill=color + (alpha,)
        )

    hx, hy = path[0]
    dx = path[0][0] - path[1][0]
    dy = path[0][1] - path[1][1]
    heading = math.atan2(dy, dx)

    draw.ellipse(
        [hx - head_r * GLOW_RADIUS[2], hy - head_r * GLOW_RADIUS[2], hx + head_r * GLOW_RADIUS[2], hy + head_r * GLOW_RADIUS[2]],
        fill=SNAKE_HEAD + (40,)
    )
    draw.ellipse(
        [hx - head_r, hy - head_r, hx + head_r, hy + head_r],
        fill=SNAKE_HEAD + (255,)
    )
    draw.ellipse(
        [hx - head_r, hy - head_r, hx + head_r, hy + head_r],
        outline=SNAKE_BODY + (255,), width=max(int(head_r * 0.15), 2)
    )

    eye_dist = head_r * 0.5
    eye_r = head_r * 0.32
    pupil_r = eye_r * 0.55
    perp = heading + math.pi / 2
    for side in [-1, 1]:
        ex = hx + math.cos(heading) * head_r * 0.3 + math.cos(perp) * eye_dist * side
        ey = hy + math.sin(heading) * head_r * 0.3 + math.sin(perp) * eye_dist * side
        draw.ellipse(
            [ex - eye_r, ey - eye_r, ex + eye_r, ey + eye_r],
            fill=EYE_WHITE
        )
        ppx = ex + math.cos(heading) * eye_r * 0.25
        ppy = ey + math.sin(heading) * eye_r * 0.25
        draw.ellipse(
            [ppx - pupil_r, ppy - pupil_r, ppx + pupil_r, ppy + pupil_r],
            fill=EYE_PUPIL
        )

    return img


def generate_favicon_svg():
    """Generate an SVG favicon (snake head on dark background)."""
    return '''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">
  <rect width="32" height="32" rx="6" fill="#0a0a2e"/>
  <circle cx="16" cy="16" r="14" fill="#ff446644"/>
  <circle cx="16" cy="16" r="10" fill="#ff4466" stroke="#cc2244" stroke-width="1.5"/>
  <circle cx="13" cy="13" r="2.8" fill="white"/>
  <circle cx="19" cy="13" r="2.8" fill="white"/>
  <circle cx="13.8" cy="12.5" r="1.5" fill="#14141e"/>
  <circle cx="19.8" cy="12.5" r="1.5" fill="#14141e"/>
  <circle cx="10" cy="24" r="1.5" fill="#ff4466" opacity="0.7"/>
  <circle cx="16" cy="26" r="1.5" fill="#cc2244" opacity="0.7"/>
  <circle cx="22" cy="24" r="1.5" fill="#ff4466" opacity="0.7"/>
</svg>'''


def _populate_imagestack(stack_dir, w1x, h1x, label):
    """Populate a 3-layer imagestack (Front/Middle/Back) at given 1x size.

    Front: snake on transparent background (RGBA)
    Middle: food particles on transparent background (RGBA)
    Back: opaque background with grid (RGB) — must be last & fully opaque
    """
    import json
    w2x, h2x = w1x * 2, h1x * 2

    # Front layer: snake on transparent background
    front_dir = os.path.join(stack_dir, "Front.imagestacklayer", "Content.imageset")
    for (w, h), scale in [((w1x, h1x), "1x"), ((w2x, h2x), "2x")]:
        img = draw_icon_rect(w, h, layer="front")
        fname = f"front_{w}x{h}.png"
        img.save(os.path.join(front_dir, fname))
        print(f"  -> {label}/Front/{fname}")
    with open(os.path.join(front_dir, "Contents.json"), "w") as f:
        json.dump({
            "images": [
                {"filename": f"front_{w1x}x{h1x}.png", "idiom": "tv", "scale": "1x"},
                {"filename": f"front_{w2x}x{h2x}.png", "idiom": "tv", "scale": "2x"},
            ],
            "info": {"version": 1, "author": "xcode"}
        }, f, indent=2)

    # Middle layer: food particles on transparent background
    middle_dir = os.path.join(stack_dir, "Middle.imagestacklayer", "Content.imageset")
    for (w, h), scale in [((w1x, h1x), "1x"), ((w2x, h2x), "2x")]:
        img = draw_icon_rect(w, h, layer="middle")
        fname = f"middle_{w}x{h}.png"
        img.save(os.path.join(middle_dir, fname))
        print(f"  -> {label}/Middle/{fname}")
    with open(os.path.join(middle_dir, "Contents.json"), "w") as f:
        json.dump({
            "images": [
                {"filename": f"middle_{w1x}x{h1x}.png", "idiom": "tv", "scale": "1x"},
                {"filename": f"middle_{w2x}x{h2x}.png", "idiom": "tv", "scale": "2x"},
            ],
            "info": {"version": 1, "author": "xcode"}
        }, f, indent=2)

    # Back layer: opaque background with grid
    back_dir = os.path.join(stack_dir, "Back.imagestacklayer", "Content.imageset")
    for (w, h), scale in [((w1x, h1x), "1x"), ((w2x, h2x), "2x")]:
        img = draw_icon_rect(w, h, layer="back")
        fname = f"back_{w}x{h}.png"
        img.save(os.path.join(back_dir, fname))
        print(f"  -> {label}/Back/{fname}")
    with open(os.path.join(back_dir, "Contents.json"), "w") as f:
        json.dump({
            "images": [
                {"filename": f"back_{w1x}x{h1x}.png", "idiom": "tv", "scale": "1x"},
                {"filename": f"back_{w2x}x{h2x}.png", "idiom": "tv", "scale": "2x"},
            ],
            "info": {"version": 1, "author": "xcode"}
        }, f, indent=2)


def main():
    root = os.path.dirname(os.path.abspath(__file__))
    assets_dir = os.path.join(root, "appletv", "SnakeTV", "Assets.xcassets")
    import json

    brand_dir = os.path.join(assets_dir, "Brand Assets.brandassets")

    # --- App Icon (Home Screen): 400x240 ---
    stack = os.path.join(brand_dir, "App Icon.imagestack")
    _populate_imagestack(stack, 400, 240, "App Icon")

    # --- App Icon - App Store: 1280x768 ---
    stack = os.path.join(brand_dir, "App Icon - App Store.imagestack")
    _populate_imagestack(stack, 1280, 768, "App Store")

    # --- Top Shelf Image: 1920x720 ---
    shelf_dir = os.path.join(brand_dir, "Top Shelf Image.imageset")
    for (w, h), scale in [((1920, 720), "1x"), ((3840, 1440), "2x")]:
        fname = f"topshelf_{w}x{h}.png"
        img = draw_icon_rect(w, h)
        img.save(os.path.join(shelf_dir, fname))
        print(f"  -> Top Shelf/{fname}")
    with open(os.path.join(shelf_dir, "Contents.json"), "w") as f:
        json.dump({
            "images": [
                {"filename": "topshelf_1920x720.png", "idiom": "tv", "scale": "1x"},
                {"filename": "topshelf_3840x1440.png", "idiom": "tv", "scale": "2x"},
            ],
            "info": {"version": 1, "author": "xcode"}
        }, f, indent=2)

    # --- Top Shelf Image Wide: 2320x720 ---
    shelf_wide_dir = os.path.join(brand_dir, "Top Shelf Image Wide.imageset")
    for (w, h), scale in [((2320, 720), "1x"), ((4640, 1440), "2x")]:
        fname = f"topshelf_wide_{w}x{h}.png"
        img = draw_icon_rect(w, h)
        img.save(os.path.join(shelf_wide_dir, fname))
        print(f"  -> Top Shelf Wide/{fname}")
    with open(os.path.join(shelf_wide_dir, "Contents.json"), "w") as f:
        json.dump({
            "images": [
                {"filename": "topshelf_wide_2320x720.png", "idiom": "tv", "scale": "1x"},
                {"filename": "topshelf_wide_4640x1440.png", "idiom": "tv", "scale": "2x"},
            ],
            "info": {"version": 1, "author": "xcode"}
        }, f, indent=2)

    # --- Square icon for web / favicon ---
    favicon_dir = os.path.join(root, "engine")
    icon32 = draw_icon(32)
    icon32.save(os.path.join(favicon_dir, "favicon.png"))
    print("  -> engine/favicon.png (32x32)")

    icon180 = draw_icon(180)
    icon180.save(os.path.join(favicon_dir, "apple-touch-icon.png"))
    print("  -> engine/apple-touch-icon.png (180x180)")

    svg = generate_favicon_svg()
    with open(os.path.join(favicon_dir, "favicon.svg"), "w") as f:
        f.write(svg)
    print("  -> engine/favicon.svg")

    print("\nDone! All icons generated.")


if __name__ == "__main__":
    main()
