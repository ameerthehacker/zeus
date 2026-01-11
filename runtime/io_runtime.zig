// Zeus IO Runtime Functions
// IO primordial runtime functions for Zeus

const std = @import("std");
const abi = @import("abi.zig");
const string_runtime = @import("string_runtime.zig");

/// zeus_log: Prints a string object to stdout followed by a newline
/// Signature: zeus_log(return_buffer_ptr, string_ptr_ptr)
export fn zeus_log(return_buffer_ptr: ?*anyopaque, string_ptr_ptr: *anyopaque) callconv(.C) void {
    _ = return_buffer_ptr;

    // Dereference to get the ZeusStringObj pointer
    const string_obj_ptr = @as(**anyopaque, @ptrCast(@alignCast(string_ptr_ptr))).*;
    const string_ptr = string_runtime.castToStringObj(string_obj_ptr);

    const stdout = std.io.getStdOut().writer();

    // Get the internal u8[] array from the string
    if (string_ptr.data) |array_ptr| {
        const length = array_ptr.length;
        if (length > 0) {
            if (array_ptr.data) |data| {
                const str_bytes = @as([*]const u8, @ptrCast(@alignCast(data)))[0..length];
                stdout.writeAll(str_bytes) catch {};
            }
        }
    }

    // Always print newline at the end
    stdout.writeAll("\n") catch {};
}
