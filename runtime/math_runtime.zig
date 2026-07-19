// Zeus Math Runtime Functions
//
// Backs the `Math` primordial (see internal/prelude/math.zs). `Math` is a pure static class, so
// each export follows the static extern-method ABI produced by codegen (emitExternMethodBody):
//   fn(this_ptr, return_buffer_ptr_ptr, ...param_ptrs)
// Params arrive as pointers to f64; f64 results are boxed via runtime_util.allocateReturnBuffer and
// read back from the wrapper's result field. `this_ptr` is always null here (static methods have no
// receiver) and ignored — the slot is kept so the ABI matches instance extern methods. The PI/E
// constants live as inline static-field initializers in math.zs, so there is no constructor.

const std = @import("std");
const runtime_util = @import("runtime_util.zig");

// ---- ABI helpers ----------------------------------------------------------

inline fn readF64(ptr: *anyopaque) f64 {
    return @as(*f64, @ptrCast(@alignCast(ptr))).*;
}

inline fn writeF64(return_buffer_ptr_ptr: ?*anyopaque, value: f64) void {
    if (runtime_util.allocateReturnBuffer(return_buffer_ptr_ptr, @sizeOf(f64))) |bytes| {
        @as(*f64, @ptrCast(@alignCast(bytes.ptr))).* = value;
    }
}

// ---- Unary functions ------------------------------------------------------

export fn zeus_Math_sqrt(this_ptr: ?*anyopaque, rb: ?*anyopaque, x_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    writeF64(rb, @sqrt(readF64(x_ptr)));
}

export fn zeus_Math_cbrt(this_ptr: ?*anyopaque, rb: ?*anyopaque, x_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    writeF64(rb, std.math.cbrt(readF64(x_ptr)));
}

export fn zeus_Math_exp(this_ptr: ?*anyopaque, rb: ?*anyopaque, x_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    writeF64(rb, @exp(readF64(x_ptr)));
}

export fn zeus_Math_log(this_ptr: ?*anyopaque, rb: ?*anyopaque, x_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    writeF64(rb, @log(readF64(x_ptr)));
}

export fn zeus_Math_log2(this_ptr: ?*anyopaque, rb: ?*anyopaque, x_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    writeF64(rb, @log2(readF64(x_ptr)));
}

export fn zeus_Math_log10(this_ptr: ?*anyopaque, rb: ?*anyopaque, x_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    writeF64(rb, @log10(readF64(x_ptr)));
}

export fn zeus_Math_sin(this_ptr: ?*anyopaque, rb: ?*anyopaque, x_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    writeF64(rb, @sin(readF64(x_ptr)));
}

export fn zeus_Math_cos(this_ptr: ?*anyopaque, rb: ?*anyopaque, x_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    writeF64(rb, @cos(readF64(x_ptr)));
}

export fn zeus_Math_tan(this_ptr: ?*anyopaque, rb: ?*anyopaque, x_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    writeF64(rb, @tan(readF64(x_ptr)));
}

export fn zeus_Math_floor(this_ptr: ?*anyopaque, rb: ?*anyopaque, x_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    writeF64(rb, @floor(readF64(x_ptr)));
}

export fn zeus_Math_ceil(this_ptr: ?*anyopaque, rb: ?*anyopaque, x_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    writeF64(rb, @ceil(readF64(x_ptr)));
}

export fn zeus_Math_round(this_ptr: ?*anyopaque, rb: ?*anyopaque, x_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    writeF64(rb, @round(readF64(x_ptr)));
}

export fn zeus_Math_trunc(this_ptr: ?*anyopaque, rb: ?*anyopaque, x_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    writeF64(rb, @trunc(readF64(x_ptr)));
}

export fn zeus_Math_abs(this_ptr: ?*anyopaque, rb: ?*anyopaque, x_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    writeF64(rb, @abs(readF64(x_ptr)));
}

export fn zeus_Math_sign(this_ptr: ?*anyopaque, rb: ?*anyopaque, x_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    const x = readF64(x_ptr);
    // Mirror JS Math.sign: -1/0/1, preserving 0.0/-0.0/NaN via the passthrough.
    const result = if (x > 0) @as(f64, 1.0) else if (x < 0) @as(f64, -1.0) else x;
    writeF64(rb, result);
}

// ---- Binary functions -----------------------------------------------------

export fn zeus_Math_pow(this_ptr: ?*anyopaque, rb: ?*anyopaque, x_ptr: *anyopaque, y_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    writeF64(rb, std.math.pow(f64, readF64(x_ptr), readF64(y_ptr)));
}

export fn zeus_Math_min(this_ptr: ?*anyopaque, rb: ?*anyopaque, a_ptr: *anyopaque, b_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    writeF64(rb, @min(readF64(a_ptr), readF64(b_ptr)));
}

export fn zeus_Math_max(this_ptr: ?*anyopaque, rb: ?*anyopaque, a_ptr: *anyopaque, b_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    writeF64(rb, @max(readF64(a_ptr), readF64(b_ptr)));
}

export fn zeus_Math_hypot(this_ptr: ?*anyopaque, rb: ?*anyopaque, a_ptr: *anyopaque, b_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    const a = readF64(a_ptr);
    const b = readF64(b_ptr);
    writeF64(rb, @sqrt(a * a + b * b));
}

// ---- Nullary functions ----------------------------------------------------

export fn zeus_Math_random(this_ptr: ?*anyopaque, rb: ?*anyopaque) callconv(.C) void {
    _ = this_ptr;
    // Uniform in [0, 1), matching JS Math.random().
    writeF64(rb, std.crypto.random.float(f64));
}

// ---- Inverse & hyperbolic trig -------------------------------------------

export fn zeus_Math_asin(this_ptr: ?*anyopaque, rb: ?*anyopaque, x_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    writeF64(rb, std.math.asin(readF64(x_ptr)));
}

export fn zeus_Math_acos(this_ptr: ?*anyopaque, rb: ?*anyopaque, x_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    writeF64(rb, std.math.acos(readF64(x_ptr)));
}

export fn zeus_Math_atan(this_ptr: ?*anyopaque, rb: ?*anyopaque, x_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    writeF64(rb, std.math.atan(readF64(x_ptr)));
}

export fn zeus_Math_atan2(this_ptr: ?*anyopaque, rb: ?*anyopaque, y_ptr: *anyopaque, x_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    writeF64(rb, std.math.atan2(readF64(y_ptr), readF64(x_ptr)));
}

export fn zeus_Math_sinh(this_ptr: ?*anyopaque, rb: ?*anyopaque, x_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    writeF64(rb, std.math.sinh(readF64(x_ptr)));
}

export fn zeus_Math_cosh(this_ptr: ?*anyopaque, rb: ?*anyopaque, x_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    writeF64(rb, std.math.cosh(readF64(x_ptr)));
}

export fn zeus_Math_tanh(this_ptr: ?*anyopaque, rb: ?*anyopaque, x_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    writeF64(rb, std.math.tanh(readF64(x_ptr)));
}

export fn zeus_Math_asinh(this_ptr: ?*anyopaque, rb: ?*anyopaque, x_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    writeF64(rb, std.math.asinh(readF64(x_ptr)));
}

export fn zeus_Math_acosh(this_ptr: ?*anyopaque, rb: ?*anyopaque, x_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    writeF64(rb, std.math.acosh(readF64(x_ptr)));
}

export fn zeus_Math_atanh(this_ptr: ?*anyopaque, rb: ?*anyopaque, x_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    writeF64(rb, std.math.atanh(readF64(x_ptr)));
}

// ---- Precision helpers ----------------------------------------------------

export fn zeus_Math_log1p(this_ptr: ?*anyopaque, rb: ?*anyopaque, x_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    writeF64(rb, std.math.log1p(readF64(x_ptr)));
}

export fn zeus_Math_expm1(this_ptr: ?*anyopaque, rb: ?*anyopaque, x_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    writeF64(rb, std.math.expm1(readF64(x_ptr)));
}

/// fround: round to the nearest 32-bit float, matching JS Math.fround.
export fn zeus_Math_fround(this_ptr: ?*anyopaque, rb: ?*anyopaque, x_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    writeF64(rb, @as(f64, @as(f32, @floatCast(readF64(x_ptr)))));
}

/// clz32: leading-zero count of the value coerced to uint32 (JS Math.clz32).
export fn zeus_Math_clz32(this_ptr: ?*anyopaque, rb: ?*anyopaque, x_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    const u: u32 = @bitCast(toInt32(readF64(x_ptr)));
    writeF64(rb, @floatFromInt(@clz(u)));
}

/// imul: 32-bit integer multiplication with wraparound (JS Math.imul).
export fn zeus_Math_imul(this_ptr: ?*anyopaque, rb: ?*anyopaque, a_ptr: *anyopaque, b_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    const product: i32 = toInt32(readF64(a_ptr)) *% toInt32(readF64(b_ptr));
    writeF64(rb, @floatFromInt(product));
}

/// JS ToInt32: truncate toward zero, then take the low 32 bits (modulo 2^32).
fn toInt32(x: f64) i32 {
    if (std.math.isNan(x) or std.math.isInf(x)) return 0;
    const truncated = @trunc(x);
    // Reduce modulo 2^32 into i64 range, then bitcast the low 32 bits.
    const m = @mod(truncated, 4294967296.0);
    const as_i64: i64 = @intFromFloat(m);
    return @bitCast(@as(u32, @truncate(@as(u64, @bitCast(as_i64)))));
}
