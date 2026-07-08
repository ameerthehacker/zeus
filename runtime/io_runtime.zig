// Zeus IO Runtime Functions
// IO primordial runtime functions for Zeus

const std = @import("std");
const abi = @import("abi.zig");
const string_runtime = @import("string_runtime.zig");

fn printStringObj(string_ptr_ptr: *anyopaque, writer: anytype) void {
    const string_obj_ptr = @as(**anyopaque, @ptrCast(@alignCast(string_ptr_ptr))).*;
    const string_ptr = string_runtime.castToStringObj(string_obj_ptr);
    if (string_ptr.data) |array_ptr| {
        const length = array_ptr.length;
        if (length > 0) {
            if (array_ptr.data) |data| {
                const str_bytes = @as([*]const u8, @ptrCast(@alignCast(data)))[0..length];
                writer.writeAll(str_bytes) catch {};
            }
        }
    }
    writer.writeAll("\n") catch {};
}

/// zeus_Console_log: console.log method — prints to stdout with newline
/// Signature: zeus_Console_log(this_ptr, return_buffer_ptr, string_ptr_ptr)
export fn zeus_Console_log(this_ptr: ?*anyopaque, return_buffer_ptr: ?*anyopaque, string_ptr_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    _ = return_buffer_ptr;
    printStringObj(string_ptr_ptr, std.io.getStdOut().writer());
}

/// zeus_Console_error: console.error method — prints to stderr with newline
/// Signature: zeus_Console_error(this_ptr, return_buffer_ptr, string_ptr_ptr)
export fn zeus_Console_error(this_ptr: ?*anyopaque, return_buffer_ptr: ?*anyopaque, string_ptr_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    _ = return_buffer_ptr;
    printStringObj(string_ptr_ptr, std.io.getStdErr().writer());
}

/// zeus_Console_info: console.info method — prints to stdout with newline
/// Signature: zeus_Console_info(this_ptr, return_buffer_ptr, string_ptr_ptr)
export fn zeus_Console_info(this_ptr: ?*anyopaque, return_buffer_ptr: ?*anyopaque, string_ptr_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    _ = return_buffer_ptr;
    printStringObj(string_ptr_ptr, std.io.getStdOut().writer());
}
