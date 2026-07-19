// Zeus boxed-primitive runtime.
// Backing methods for the per-type numeric boxes (prelude/boxes.zs) and the boolean box
// (prelude/bool.zs). A box is laid out as [obj_header, value: T]; the autobox lowering stores the
// scalar directly into `value`, so there is no constructor — only these method bodies. The numeric
// toString/valueOf are generated for every scalar type by the comptime loop at the bottom.

const std = @import("std");
const abi = @import("abi.zig");
const runtime_util = @import("runtime_util.zig");

// Factory functions synthesized by codegen for the string path (declared here for runtime use).
extern fn zeus_new_u8_array(capacity: i32) *anyopaque;
extern fn zeus_new_string(data: *anyopaque) *anyopaque;

/// Bool object layout — matches the LLVM struct for the Bool class (prelude/bool.zs).
pub const ZeusBoolObj = extern struct {
    obj_header: *abi.ZeusObjectHeader,
    value: bool,
};

inline fn castToBoolObj(ptr: *anyopaque) *ZeusBoolObj {
    return @as(*ZeusBoolObj, @ptrCast(@alignCast(ptr)));
}

/// Build a Zeus string object from raw bytes, mirroring zeus_string_concat: allocate a u8[] of the
/// right length, copy the bytes in, then wrap it in a string via the codegen-synthesized factory.
pub fn makeString(text: []const u8) *anyopaque {
    const len: i32 = @intCast(text.len);
    const array_ptr = zeus_new_u8_array(len);
    const array = runtime_util.castToArrayObj(array_ptr);
    if (array.data) |dest_data| {
        const dest = @as([*]u8, @ptrCast(@alignCast(dest_data)));
        @memcpy(dest[0..text.len], text);
        array.length = @intCast(len);
    }
    return zeus_new_string(array_ptr);
}

/// Write a returned string-object pointer through the Zeus return-wrapper ABI.
pub inline fn writeStringResult(return_buffer_ptr_ptr: ?*anyopaque, str_ptr: *anyopaque) void {
    if (runtime_util.allocateReturnBuffer(return_buffer_ptr_ptr, @sizeOf(*anyopaque))) |result_bytes| {
        const result_ptr = @as(**anyopaque, @ptrCast(@alignCast(result_bytes.ptr)));
        result_ptr.* = str_ptr;
    }
}

/// Write an f64 return value through the Zeus return-wrapper ABI (used by valueOf).
inline fn writeF64Result(return_buffer_ptr_ptr: ?*anyopaque, v: f64) void {
    if (runtime_util.allocateReturnBuffer(return_buffer_ptr_ptr, @sizeOf(f64))) |result_bytes| {
        @as(*f64, @ptrCast(@alignCast(result_bytes.ptr))).* = v;
    }
}

/// Format a float of type T the way a boxed float stringifies: whole values print without a decimal
/// point (5 -> "5"); other values use decimal notation. Formatting is done at the box's own width
/// (T), NOT widened to f64 — so an f32 prints its shortest f32 form (0.1 -> "0.1", not the f64
/// expansion 0.10000000149...). An extreme magnitude whose positional decimal would overflow buf
/// falls back to scientific notation, which is always short.
pub fn formatFloat(comptime T: type, buf: []u8, value: T) []const u8 {
    if (std.math.isNan(value)) {
        return std.fmt.bufPrint(buf, "NaN", .{}) catch unreachable;
    }
    if (std.math.isInf(value)) {
        return std.fmt.bufPrint(buf, "{s}", .{if (value > 0) "Infinity" else "-Infinity"}) catch unreachable;
    }
    if (std.math.floor(value) == value and @abs(value) < 1e15) {
        const as_int: i64 = @intFromFloat(value);
        return std.fmt.bufPrint(buf, "{d}", .{as_int}) catch unreachable;
    }
    return std.fmt.bufPrint(buf, "{d}", .{value}) catch
        (std.fmt.bufPrint(buf, "{e}", .{value}) catch unreachable);
}

/// Format a scalar of type T. Integers are exact ({d}); floats format at their native width.
pub fn formatScalar(comptime T: type, buf: []u8, value: T) []const u8 {
    return switch (@typeInfo(T)) {
        .Int => std.fmt.bufPrint(buf, "{d}", .{value}) catch unreachable,
        .Float => formatFloat(T, buf, value),
        else => unreachable,
    };
}

/// Bool.toString(): string
export fn zeus_bool_toString(this_ptr: *anyopaque, return_buffer_ptr_ptr: ?*anyopaque) callconv(.C) void {
    const b = castToBoolObj(this_ptr);
    const text: []const u8 = if (b.value) "true" else "false";
    writeStringResult(return_buffer_ptr_ptr, makeString(text));
}

// The scalar types with a per-type numeric box (prelude/boxes.zs). Each gets an exact toString and a
// valueOf that widens to f64.
const box_types = .{
    .{ "i8", i8 },   .{ "i16", i16 }, .{ "i32", i32 }, .{ "i64", i64 },
    .{ "u8", u8 },   .{ "u16", u16 }, .{ "u32", u32 }, .{ "u64", u64 },
    .{ "f32", f32 }, .{ "f64", f64 },
};

comptime {
    for (box_types) |bt| {
        const name = bt[0];
        const T = bt[1];
        const Box = extern struct {
            obj_header: *abi.ZeusObjectHeader,
            value: T,
        };
        const gen = struct {
            fn toString(this_ptr: *anyopaque, return_buffer_ptr_ptr: ?*anyopaque) callconv(.C) void {
                const box: *Box = @ptrCast(@alignCast(this_ptr));
                // Large enough for any f64 positional decimal (subnormals need ~326 chars); formatFloat
                // also falls back to scientific if a value somehow needs more.
                var buf: [512]u8 = undefined;
                writeStringResult(return_buffer_ptr_ptr, makeString(formatScalar(T, &buf, box.value)));
            }
            fn valueOf(this_ptr: *anyopaque, return_buffer_ptr_ptr: ?*anyopaque) callconv(.C) void {
                const box: *Box = @ptrCast(@alignCast(this_ptr));
                const v: f64 = switch (@typeInfo(T)) {
                    .Int => @floatFromInt(box.value),
                    .Float => @floatCast(box.value),
                    else => unreachable,
                };
                writeF64Result(return_buffer_ptr_ptr, v);
            }
        };
        @export(gen.toString, .{ .name = "zeus_box_" ++ name ++ "_toString" });
        @export(gen.valueOf, .{ .name = "zeus_box_" ++ name ++ "_valueOf" });
    }
}

// ============================================================================
// Global number parsing / predicates (JS globals: parseInt/parseFloat/isNaN/
// isFinite). Ambient primordial free functions (see internal/prelude/numparse.zs),
// so they use the fat ABI: (return_buffer_ptr, ...arg_ptr_ptrs). parseInt/parseFloat
// return f64 (NaN on failure) to match JS's Number-typed result.
// ============================================================================

const string_runtime = @import("string_runtime.zig");

fn isSpace(c: u8) bool {
    return c == ' ' or c == '\t' or c == '\n' or c == '\r' or c == 0x0B or c == 0x0C;
}

fn skipWs(b: []const u8) []const u8 {
    var i: usize = 0;
    while (i < b.len and isSpace(b[i])) : (i += 1) {}
    return b[i..];
}

/// Digit value for a base up to 36, or -1 if not a digit.
fn digitVal(c: u8) i32 {
    if (c >= '0' and c <= '9') return @as(i32, c - '0');
    if (c >= 'a' and c <= 'z') return @as(i32, c - 'a') + 10;
    if (c >= 'A' and c <= 'Z') return @as(i32, c - 'A') + 10;
    return -1;
}

/// JS-lenient parseInt: skip whitespace, optional sign, optional 0x for base 16, then consume valid
/// digits until the first invalid one. NaN if no digit was consumed. Base 10 unless a 0x prefix.
fn parseIntLenient(input: []const u8) f64 {
    var b = skipWs(input);
    var neg = false;
    if (b.len > 0 and (b[0] == '+' or b[0] == '-')) {
        neg = b[0] == '-';
        b = b[1..];
    }
    var radix: i32 = 10;
    if (b.len >= 2 and b[0] == '0' and (b[1] == 'x' or b[1] == 'X')) {
        radix = 16;
        b = b[2..];
    }
    var acc: f64 = 0;
    var any = false;
    for (b) |c| {
        const d = digitVal(c);
        if (d < 0 or d >= radix) break;
        acc = acc * @as(f64, @floatFromInt(radix)) + @as(f64, @floatFromInt(d));
        any = true;
    }
    if (!any) return std.math.nan(f64);
    return if (neg) -acc else acc;
}

/// JS-lenient parseFloat: parse the longest leading decimal/scientific prefix; NaN if none.
fn parseFloatLenient(input: []const u8) f64 {
    const b = skipWs(input);
    var i: usize = 0;
    if (i < b.len and (b[i] == '+' or b[i] == '-')) i += 1;
    var seen_digit = false;
    var seen_dot = false;
    while (i < b.len) : (i += 1) {
        const c = b[i];
        if (c >= '0' and c <= '9') {
            seen_digit = true;
        } else if (c == '.' and !seen_dot) {
            seen_dot = true;
        } else break;
    }
    if (seen_digit and i < b.len and (b[i] == 'e' or b[i] == 'E')) {
        var j = i + 1;
        if (j < b.len and (b[j] == '+' or b[j] == '-')) j += 1;
        var exp_digits = false;
        while (j < b.len and b[j] >= '0' and b[j] <= '9') : (j += 1) exp_digits = true;
        if (exp_digits) i = j;
    }
    if (!seen_digit) return std.math.nan(f64);
    return std.fmt.parseFloat(f64, b[0..i]) catch std.math.nan(f64);
}

// Direct C ABI (bound in the prelude via @extern("zeus", "...")): the string arrives as its object
// pointer and results return by value — the same proven marshalling the c_ffi primitives use.
pub export fn zeus_parse_int(s_obj: ?*anyopaque) callconv(.C) f64 {
    return parseIntLenient(string_runtime.zeusStringBytes(s_obj));
}

pub export fn zeus_parse_float(s_obj: ?*anyopaque) callconv(.C) f64 {
    return parseFloatLenient(string_runtime.zeusStringBytes(s_obj));
}

pub export fn zeus_is_nan(x: f64) callconv(.C) bool {
    return std.math.isNan(x);
}

pub export fn zeus_is_finite(x: f64) callconv(.C) bool {
    return std.math.isFinite(x);
}
