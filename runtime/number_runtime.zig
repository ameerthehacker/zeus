// Zeus Number/Bool Runtime Functions
// Backing methods for the boxed-primitive primordials (prelude/number.zs, prelude/bool.zs). A Number
// wraps an f64, a Bool wraps a bool; both are laid out as [obj_header, value]. The autobox lowering
// stores the scalar directly into `value`, so there is no constructor — only these method bodies.

const std = @import("std");
const abi = @import("abi.zig");
const runtime_util = @import("runtime_util.zig");

// Factory functions synthesized by codegen for the string path (declared here for runtime use).
extern fn zeus_new_u8_array(capacity: i32) *anyopaque;
extern fn zeus_new_string(data: *anyopaque) *anyopaque;

/// Number object layout — matches the LLVM struct for the Number class (prelude/number.zs):
/// - obj_header (implicit)
/// - value (f64)
pub const ZeusNumberObj = extern struct {
    obj_header: *abi.ZeusObjectHeader,
    value: f64,
};

/// Bool object layout — matches the LLVM struct for the Bool class (prelude/bool.zs).
pub const ZeusBoolObj = extern struct {
    obj_header: *abi.ZeusObjectHeader,
    value: bool,
};

inline fn castToNumberObj(ptr: *anyopaque) *ZeusNumberObj {
    return @as(*ZeusNumberObj, @ptrCast(@alignCast(ptr)));
}

inline fn castToBoolObj(ptr: *anyopaque) *ZeusBoolObj {
    return @as(*ZeusBoolObj, @ptrCast(@alignCast(ptr)));
}

/// Build a Zeus string object from raw bytes, mirroring zeus_string_concat: allocate a u8[] of the
/// right length, copy the bytes in, then wrap it in a string via the codegen-synthesized factory.
fn makeString(text: []const u8) *anyopaque {
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
inline fn writeStringResult(return_buffer_ptr_ptr: ?*anyopaque, str_ptr: *anyopaque) void {
    if (runtime_util.allocateReturnBuffer(return_buffer_ptr_ptr, @sizeOf(*anyopaque))) |result_bytes| {
        const result_ptr = @as(**anyopaque, @ptrCast(@alignCast(result_bytes.ptr)));
        result_ptr.* = str_ptr;
    }
}

/// Format an f64 the way a boxed Number stringifies: whole values print without a decimal point
/// (5 -> "5", not "5.0"); non-integral values use decimal notation. Values above 2^53 lose integer
/// exactness (the box stores an f64) — a documented v1 limitation.
fn formatNumber(buf: []u8, value: f64) []const u8 {
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
    return std.fmt.bufPrint(buf, "{d}", .{value}) catch unreachable;
}

/// Number.toString(): string
export fn zeus_number_toString(this_ptr: *anyopaque, return_buffer_ptr_ptr: ?*anyopaque) callconv(.C) void {
    const num = castToNumberObj(this_ptr);
    var buf: [64]u8 = undefined;
    const text = formatNumber(&buf, num.value);
    writeStringResult(return_buffer_ptr_ptr, makeString(text));
}

/// Bool.toString(): string
export fn zeus_bool_toString(this_ptr: *anyopaque, return_buffer_ptr_ptr: ?*anyopaque) callconv(.C) void {
    const b = castToBoolObj(this_ptr);
    const text: []const u8 = if (b.value) "true" else "false";
    writeStringResult(return_buffer_ptr_ptr, makeString(text));
}
