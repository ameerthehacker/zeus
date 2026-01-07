// Zeus IO Runtime Functions
// IO primordial runtime functions for Zeus

const std = @import("std");
const abi = @import("abi.zig");
const runtime_util = @import("runtime_util.zig");

const castToArrayObj = runtime_util.castToArrayObj;

/// zeus_log: Prints a u8[] array as a UTF-8 string to stdout
/// Signature: zeus_log(return_buffer_ptr, str_array_ptr_ptr)
export fn zeus_log(return_buffer_ptr: ?*anyopaque, str_array_ptr_ptr: *anyopaque) callconv(.C) void {
    _ = return_buffer_ptr;

    // Dereference to get the ZeusArrayObj pointer
    const array_obj_ptr = @as(**anyopaque, @ptrCast(@alignCast(str_array_ptr_ptr))).*;
    const array_ptr = castToArrayObj(array_obj_ptr);

    const length = array_ptr.length;
    if (length == 0) return;

    if (array_ptr.data) |data| {
        const str_bytes = @as([*]const u8, @ptrCast(@alignCast(data)))[0..length];
        const stdout = std.io.getStdOut().writer();
        stdout.writeAll(str_bytes) catch {};
    }
}

