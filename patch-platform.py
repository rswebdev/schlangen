#!/usr/bin/env python3
"""Patch Mach-O object files to change platform from iOS to tvOS.

Modifies the LC_BUILD_VERSION load command:
  platform 2 (iOS)           -> 3 (tvOS)
  platform 6 (iOS Simulator) -> 7 (tvOS Simulator)

Works on .o files and thin (non-fat) archives.
For fat binaries, extract with lipo first.
"""
import struct
import sys
import os

# Mach-O constants
MH_MAGIC_64 = 0xFEEDFACF
LC_BUILD_VERSION = 0x32

PLATFORM_IOS = 2
PLATFORM_TVOS = 3
PLATFORM_IOS_SIM = 7
PLATFORM_TVOS_SIM = 8


def patch_macho(data, target_platform):
    """Patch LC_BUILD_VERSION platform in a Mach-O binary. Returns patched bytes."""
    buf = bytearray(data)

    # Check magic
    if len(buf) < 32:
        return buf

    magic = struct.unpack_from('<I', buf, 0)[0]
    if magic != MH_MAGIC_64:
        return buf  # Not a 64-bit Mach-O

    # Header: magic(4) cputype(4) cpusubtype(4) filetype(4)
    #         ncmds(4) sizeofcmds(4) flags(4) reserved(4)
    ncmds = struct.unpack_from('<I', buf, 16)[0]

    offset = 32  # Start of load commands (after header)
    patched = 0

    for _ in range(ncmds):
        if offset + 8 > len(buf):
            break
        cmd, cmdsize = struct.unpack_from('<II', buf, offset)

        if cmd == LC_BUILD_VERSION and cmdsize >= 16:
            platform = struct.unpack_from('<I', buf, offset + 8)[0]
            if platform == PLATFORM_IOS and target_platform in ('tvos', 3):
                struct.pack_into('<I', buf, offset + 8, PLATFORM_TVOS)
                patched += 1
            elif platform == PLATFORM_IOS_SIM and target_platform in ('tvos-sim', 7):
                struct.pack_into('<I', buf, offset + 8, PLATFORM_TVOS_SIM)
                patched += 1

        offset += cmdsize

    return bytes(buf), patched


def patch_archive(archive_path, output_path, target_platform):
    """Patch all .o files inside an ar archive."""
    with open(archive_path, 'rb') as f:
        data = f.read()

    # Check ar magic
    if not data.startswith(b'!<arch>\n'):
        print(f"  Not an ar archive: {archive_path}")
        return False

    buf = bytearray(data)
    pos = 8  # Skip magic
    total_patched = 0
    obj_count = 0

    while pos < len(buf):
        # AR header: name(16) mtime(12) uid(6) gid(6) mode(8) size(10) end(2)
        if pos + 60 > len(buf):
            break

        header = buf[pos:pos+60]
        if header[58:60] != b'`\n':
            break

        size_str = header[48:58].strip()
        size = int(size_str)
        obj_start = pos + 60
        obj_end = obj_start + size

        # Handle BSD long filenames: name field starts with "#1/"
        # The number after "#1/" is the length of the real name embedded
        # at the start of the data area.
        name_field = header[0:16].decode('ascii', errors='replace').strip()
        data_start = obj_start
        if name_field.startswith('#1/'):
            name_len = int(name_field[3:])
            data_start = obj_start + name_len

        # Check if this entry is a Mach-O object
        if data_start + 4 <= len(buf):
            entry_magic = struct.unpack_from('<I', buf, data_start)[0]
            if entry_magic == MH_MAGIC_64:
                obj_data = bytes(buf[data_start:obj_end])
                patched_data, count = patch_macho(obj_data, target_platform)
                if count > 0:
                    buf[data_start:obj_end] = patched_data
                    total_patched += count
                    obj_count += 1

        # Align to 2 bytes
        pos = obj_end
        if pos % 2 != 0:
            pos += 1

    with open(output_path, 'wb') as f:
        f.write(bytes(buf))

    print(f"  Patched {total_patched} LC_BUILD_VERSION commands in {obj_count} objects")
    return total_patched > 0


def patch_file(input_path, output_path, target_platform):
    """Patch a single .o file or an ar archive."""
    with open(input_path, 'rb') as f:
        magic = f.read(8)

    if magic.startswith(b'!<arch>'):
        return patch_archive(input_path, output_path, target_platform)
    elif struct.unpack('<I', magic[:4])[0] == MH_MAGIC_64:
        with open(input_path, 'rb') as f:
            data = f.read()
        patched, count = patch_macho(data, target_platform)
        with open(output_path, 'wb') as f:
            f.write(patched)
        print(f"  Patched {count} LC_BUILD_VERSION commands")
        return count > 0
    else:
        print(f"  Unknown file format: {input_path}")
        return False


if __name__ == '__main__':
    if len(sys.argv) != 4:
        print(f"Usage: {sys.argv[0]} <input> <output> <tvos|tvos-sim>")
        sys.exit(1)

    input_path, output_path, target = sys.argv[1], sys.argv[2], sys.argv[3]

    if target not in ('tvos', 'tvos-sim'):
        print(f"Invalid target: {target} (use 'tvos' or 'tvos-sim')")
        sys.exit(1)

    if not os.path.exists(input_path):
        print(f"File not found: {input_path}")
        sys.exit(1)

    if patch_file(input_path, output_path, target):
        print(f"  -> {output_path}")
    else:
        print("  No patches applied")
        sys.exit(1)
