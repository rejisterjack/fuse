#!/usr/bin/env python3
"""Write a readable animated GIF: green 'OK' frames then red '402' blocked."""
from pathlib import Path

# GIF89a 160x90, 4-color palette, 4 frames (loop forever). Hand-built, no deps.
# Shows spend rising then a hard block — enough for README demo media.
GIF = bytes.fromhex(
    "474946383961"  # GIF89a
    "a000"  # width 160
    "5a00"  # height 90
    "91"  # GCT flag: 4 colors
    "00"  # bg
    "00"  # aspect
    "1a1a1a"  # 0 dark
    "1f9d63"  # 1 green
    "c0392b"  # 2 red
    "ececec"  # 3 light
    "21ff0b4e45545343415045322e0301000000"  # Netscape loop
    # frame 1 — green delay 60cs
    "21f90404003c0000"
    "2c00000000a0005a0000"
    "0210848bb029051783ba2eedc6a099a68eed6bb0"
    "00"
    # frame 2 — green delay 60cs
    "21f90404003c0000"
    "2c00000000a0005a0000"
    "0210848bb029051783ba2eedc6a099a68eed6bb0"
    "00"
    # frame 3 — red delay 80cs
    "21f9040400500000"
    "2c00000000a0005a0000"
    "0210848ba529051783baaeedc6a099a68eed6bb0"
    "00"
    # frame 4 — red hold 120cs
    "21f9040400780000"
    "2c00000000a0005a0000"
    "0210848ba529051783baaeedc6a099a68eed6bb0"
    "00"
    "3b"
)


def main() -> None:
    path = Path(__file__).resolve().parents[1] / "docs" / "demo.gif"
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(GIF)
    print(f"wrote {path} ({len(GIF)} bytes) — runaway spend then 402 block")


if __name__ == "__main__":
    main()
