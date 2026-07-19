// Zeus encoding runtime — hex and base64, backing @std/encoding (and reused by @std/buffer and
// @std/crypto). Direct C ABI (bound via @extern("zeus", "...")): a data pointer + length in, encoded
// bytes written to a caller-provided buffer, encoded/decoded length returned (-1 on decode error).

const std = @import("std");

fn inSlice(ptr: ?*anyopaque, len: i64) []const u8 {
    if (ptr == null or len <= 0) return &[_]u8{};
    return @as([*]const u8, @ptrCast(ptr.?))[0..@intCast(len)];
}

fn outSlice(ptr: *anyopaque, cap: usize) []u8 {
    return @as([*]u8, @ptrCast(ptr))[0..cap];
}

fn hexVal(c: u8) i32 {
    if (c >= '0' and c <= '9') return @as(i32, c - '0');
    if (c >= 'a' and c <= 'f') return @as(i32, c - 'a') + 10;
    if (c >= 'A' and c <= 'F') return @as(i32, c - 'A') + 10;
    return -1;
}

const hex_chars = "0123456789abcdef";

/// Lowercase-hex encode `len` bytes into `out` (needs 2*len bytes). Returns the hex length.
pub export fn zeus_hex_encode(data: ?*anyopaque, len: i64, out: *anyopaque) callconv(.C) i64 {
    const src = inSlice(data, len);
    const dst = outSlice(out, src.len * 2);
    var i: usize = 0;
    for (src) |b| {
        dst[i] = hex_chars[b >> 4];
        dst[i + 1] = hex_chars[b & 0x0F];
        i += 2;
    }
    return @intCast(i);
}

/// Decode a hex string into `out` (needs len/2 bytes). Returns byte length, or -1 if invalid.
pub export fn zeus_hex_decode(s: ?*anyopaque, len: i64, out: *anyopaque) callconv(.C) i64 {
    const src = inSlice(s, len);
    if (src.len % 2 != 0) return -1;
    const dst = outSlice(out, src.len / 2);
    var i: usize = 0;
    while (i < src.len) : (i += 2) {
        const hi = hexVal(src[i]);
        const lo = hexVal(src[i + 1]);
        if (hi < 0 or lo < 0) return -1;
        dst[i / 2] = @intCast(hi * 16 + lo);
    }
    return @intCast(src.len / 2);
}

/// Standard base64 (with padding) encode into `out`. Returns the encoded length.
pub export fn zeus_base64_encode(data: ?*anyopaque, len: i64, out: *anyopaque) callconv(.C) i64 {
    const src = inSlice(data, len);
    const enc = std.base64.standard.Encoder;
    const n = enc.calcSize(src.len);
    const result = enc.encode(outSlice(out, n), src);
    return @intCast(result.len);
}

/// Decode standard base64 into `out`. Returns byte length, or -1 if invalid.
pub export fn zeus_base64_decode(s: ?*anyopaque, len: i64, out: *anyopaque) callconv(.C) i64 {
    const src = inSlice(s, len);
    const dec = std.base64.standard.Decoder;
    const n = dec.calcSizeForSlice(src) catch return -1;
    dec.decode(outSlice(out, n), src) catch return -1;
    return @intCast(n);
}
