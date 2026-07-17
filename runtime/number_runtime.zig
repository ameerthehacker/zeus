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
fn formatFloat(comptime T: type, buf: []u8, value: T) []const u8 {
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
fn formatScalar(comptime T: type, buf: []u8, value: T) []const u8 {
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
