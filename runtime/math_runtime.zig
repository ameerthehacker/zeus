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
