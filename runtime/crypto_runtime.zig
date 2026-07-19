// Zeus crypto runtime — backs @std/crypto, entirely on Zig's std.crypto (no OpenSSL/external deps,
// cross-platform). Direct C ABI (bound via @extern("zeus", "...")): raw bytes in, raw digest/derived
// bytes written to a caller buffer, length returned. The @std/crypto module hex/base64-encodes.
//
// algo codes: 0=md5 1=sha1 2=sha224 3=sha256 4=sha384 5=sha512.

const std = @import("std");

const Md5 = std.crypto.hash.Md5;
const Sha1 = std.crypto.hash.Sha1;
const Sha224 = std.crypto.hash.sha2.Sha224;
const Sha256 = std.crypto.hash.sha2.Sha256;
const Sha384 = std.crypto.hash.sha2.Sha384;
const Sha512 = std.crypto.hash.sha2.Sha512;
const hmac = std.crypto.auth.hmac;

fn inSlice(ptr: ?*anyopaque, len: i64) []const u8 {
    if (ptr == null or len <= 0) return &[_]u8{};
    return @as([*]const u8, @ptrCast(ptr.?))[0..@intCast(len)];
}

// ---- hashing --------------------------------------------------------------

fn hashInto(comptime H: type, data: []const u8, out: [*]u8) usize {
    var digest: [H.digest_length]u8 = undefined;
    var h = H.init(.{});
    h.update(data);
    h.final(&digest);
    @memcpy(out[0..H.digest_length], &digest);
    return H.digest_length;
}

pub export fn zeus_crypto_hash(algo: i32, data_ptr: ?*anyopaque, data_len: i64, out_ptr: *anyopaque) callconv(.C) i64 {
    const data = inSlice(data_ptr, data_len);
    const out = @as([*]u8, @ptrCast(out_ptr));
    return @intCast(switch (algo) {
        0 => hashInto(Md5, data, out),
        1 => hashInto(Sha1, data, out),
        2 => hashInto(Sha224, data, out),
        3 => hashInto(Sha256, data, out),
        4 => hashInto(Sha384, data, out),
        5 => hashInto(Sha512, data, out),
        else => @as(usize, 0),
    });
}

// ---- HMAC -----------------------------------------------------------------

fn hmacInto(comptime H: type, key: []const u8, data: []const u8, out: [*]u8) usize {
    const M = hmac.Hmac(H);
    var mac: [M.mac_length]u8 = undefined;
    M.create(&mac, data, key);
    @memcpy(out[0..M.mac_length], &mac);
    return M.mac_length;
}

pub export fn zeus_crypto_hmac(algo: i32, key_ptr: ?*anyopaque, key_len: i64, data_ptr: ?*anyopaque, data_len: i64, out_ptr: *anyopaque) callconv(.C) i64 {
    const key = inSlice(key_ptr, key_len);
    const data = inSlice(data_ptr, data_len);
    const out = @as([*]u8, @ptrCast(out_ptr));
    return @intCast(switch (algo) {
        0 => hmacInto(Md5, key, data, out),
        1 => hmacInto(Sha1, key, data, out),
        2 => hmacInto(Sha224, key, data, out),
        3 => hmacInto(Sha256, key, data, out),
        4 => hmacInto(Sha384, key, data, out),
        5 => hmacInto(Sha512, key, data, out),
        else => @as(usize, 0),
    });
}

// ---- PBKDF2 ---------------------------------------------------------------

fn pbkdf2Into(comptime H: type, out: []u8, pw: []const u8, salt: []const u8, iters: u32) bool {
    std.crypto.pwhash.pbkdf2(out, pw, salt, iters, hmac.Hmac(H)) catch return false;
    return true;
}

pub export fn zeus_crypto_pbkdf2(algo: i32, pw_ptr: ?*anyopaque, pw_len: i64, salt_ptr: ?*anyopaque, salt_len: i64, iters: i32, keylen: i32, out_ptr: *anyopaque) callconv(.C) i64 {
    if (keylen <= 0 or iters <= 0) return -1;
    const pw = inSlice(pw_ptr, pw_len);
    const salt = inSlice(salt_ptr, salt_len);
    const out = @as([*]u8, @ptrCast(out_ptr))[0..@intCast(keylen)];
    const rounds: u32 = @intCast(iters);
    const ok = switch (algo) {
        0 => pbkdf2Into(Md5, out, pw, salt, rounds),
        1 => pbkdf2Into(Sha1, out, pw, salt, rounds),
        2 => pbkdf2Into(Sha224, out, pw, salt, rounds),
        3 => pbkdf2Into(Sha256, out, pw, salt, rounds),
        4 => pbkdf2Into(Sha384, out, pw, salt, rounds),
        5 => pbkdf2Into(Sha512, out, pw, salt, rounds),
        else => false,
    };
    return if (ok) @intCast(keylen) else -1;
}

// ---- randomness -----------------------------------------------------------

pub export fn zeus_crypto_random_bytes(out_ptr: *anyopaque, n: i32) callconv(.C) void {
    if (n <= 0) return;
    std.crypto.random.bytes(@as([*]u8, @ptrCast(out_ptr))[0..@intCast(n)]);
}

/// Uniform random int in [min, max) (Node's crypto.randomInt). Returns min if the range is empty.
pub export fn zeus_crypto_random_int(min: i32, max: i32) callconv(.C) i32 {
    if (max <= min) return min;
    return std.crypto.random.intRangeLessThan(i32, min, max);
}

/// Write a RFC 4122 v4 UUID (36 chars) to `out_ptr`.
pub export fn zeus_crypto_random_uuid(out_ptr: *anyopaque) callconv(.C) void {
    var bytes: [16]u8 = undefined;
    std.crypto.random.bytes(&bytes);
    bytes[6] = (bytes[6] & 0x0f) | 0x40; // version 4
    bytes[8] = (bytes[8] & 0x3f) | 0x80; // variant 1
    const dst = @as([*]u8, @ptrCast(out_ptr))[0..36];
    const hex_chars = "0123456789abcdef";
    var di: usize = 0;
    for (bytes, 0..) |b, i| {
        if (i == 4 or i == 6 or i == 8 or i == 10) {
            dst[di] = '-';
            di += 1;
        }
        dst[di] = hex_chars[b >> 4];
        dst[di + 1] = hex_chars[b & 0x0f];
        di += 2;
    }
}

// ---- constant-time compare ------------------------------------------------

pub export fn zeus_crypto_timing_safe_equal(a_ptr: ?*anyopaque, a_len: i64, b_ptr: ?*anyopaque, b_len: i64) callconv(.C) i32 {
    const a = inSlice(a_ptr, a_len);
    const b = inSlice(b_ptr, b_len);
    if (a.len != b.len) return 0;
    var diff: u8 = 0;
    for (a, b) |x, y| diff |= x ^ y;
    return if (diff == 0) 1 else 0;
}
