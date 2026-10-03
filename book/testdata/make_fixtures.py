#!/usr/bin/env python3
"""Regenerates the CBR and CB7 test fixtures.

RAR is written by hand as a minimal RAR5 archive with stored (uncompressed)
entries, since no free tool can create RAR files. CB7 needs the `7z` command.

Run from this directory: python3 make_fixtures.py
"""
import os
import struct
import subprocess
import tempfile
import zlib

ENTRIES = [
    ("p10.png", b"ten"),
    ("p2.png", b"two"),
    ("notes.txt", b"not a page"),
    ("ComicInfo.xml", b"<ComicInfo><Title>Fixture</Title><Manga>YesAndRightToLeft</Manga></ComicInfo>"),
]


def vint(n):
    out = bytearray()
    while True:
        b = n & 0x7F
        n >>= 7
        if n:
            out.append(b | 0x80)
        else:
            out.append(b)
            return bytes(out)


def block(body):
    # body starts at the header type field; prefix with size, then CRC32.
    sized = vint(len(body)) + body
    return struct.pack("<I", zlib.crc32(sized)) + sized


def make_rar(path):
    out = bytearray(b"Rar!\x1a\x07\x01\x00")
    out += block(vint(1) + vint(0) + vint(0))  # main header
    for name, data in ENTRIES:
        n = name.encode()
        body = (
            vint(2)                   # type: file
            + vint(0x02)              # header flags: data area present
            + vint(len(data))         # data size
            + vint(0x04)              # file flags: CRC32 present
            + vint(len(data))         # unpacked size
            + vint(0)                 # attributes
            + struct.pack("<I", zlib.crc32(data))
            + vint(0)                 # compression: version 0, store
            + vint(0)                 # host OS: Windows
            + vint(len(n)) + n
        )
        out += block(body) + data
    out += block(vint(5) + vint(0) + vint(0))  # end of archive
    with open(path, "wb") as f:
        f.write(out)


def make_7z(path):
    with tempfile.TemporaryDirectory() as tmp:
        for name, data in ENTRIES:
            with open(os.path.join(tmp, name), "wb") as f:
                f.write(data)
        if os.path.exists(path):
            os.remove(path)
        subprocess.run(["7z", "a", "-ms=on", "-bd", os.path.abspath(path)] + [n for n, _ in ENTRIES],
                       cwd=tmp, check=True, stdout=subprocess.DEVNULL)


make_rar("fixture.cbr")
make_7z("fixture.cb7")
