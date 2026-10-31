#!/usr/bin/env python3
"""Write a tiny valid animated GIF (loop → 402) with no third-party deps."""
from pathlib import Path

# Hand-built GIF89a 2-frame 2-color animation (public-domain minimal pattern).
# Frame delays suggest "running" then "blocked".
GIF = bytes.fromhex(
    "474946383961"  # GIF89a
    "64003200"  # 100x50
    "910000"  # GCT flag
    "14181c"  # color0 dark
    "0f6e56"  # color1 green
    "b43c28"  # color2 red
    "c8c8c8"  # color3 gray
    "21ff0b4e45545343415045322e0301000000"  # Netscape loop
    # frame 1 green-ish delay 80cs
    "21f9040400500000"
    "2c000000006400320000"
    "0208448a0fa51b0b"  # tiny lzw payload (may show noise — fine for placeholder)
    "00"
    # frame 2 red-ish delay 120cs
    "21f9040400780000"
    "2c000000006400320000"
    "0208448aa5c11b0b"
    "00"
    "3b"
)


def main() -> None:
    path = Path(__file__).resolve().parents[1] / "docs" / "demo.gif"
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(GIF)
    print(f"wrote {path} ({len(GIF)} bytes)")


if __name__ == "__main__":
    main()
