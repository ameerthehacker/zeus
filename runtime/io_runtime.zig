// Zeus IO Runtime Functions
// IO primordial runtime functions for Zeus

const std = @import("std");
const abi = @import("abi.zig");
const string_runtime = @import("string_runtime.zig");

/// Write a Zeus string object's bytes to the writer (no newline).
fn writeStringObj(str_obj_ptr: *anyopaque, writer: anytype) void {
    const string_ptr = string_runtime.castToStringObj(str_obj_ptr);
    if (string_ptr.data) |array_ptr| {
        if (array_ptr.length > 0) {
            if (array_ptr.data) |data| {
                const str_bytes = @as([*]const u8, @ptrCast(@alignCast(data)))[0..array_ptr.length];
                writer.writeAll(str_bytes) catch {};
            }
        }
    }
}

/// Print the variadic string[] args joined by a single space, then a newline. `args_ptr_ptr` points at
/// the packed string[] array (the variadic lowering collects console.log(a, b, c) into one array whose
/// elements are string-object pointers). One arg reproduces the old single-string output byte-for-byte;
/// zero args → just "\n".
fn printArgs(args_ptr_ptr: *anyopaque, writer: anytype) void {
    const array_obj = @as(**anyopaque, @ptrCast(@alignCast(args_ptr_ptr))).*;
    const array = @as(*abi.ZeusArrayObj, @ptrCast(@alignCast(array_obj)));
    if (array.data) |data| {
        const elems = @as([*]const ?*anyopaque, @ptrCast(@alignCast(data)));
        var i: u32 = 0;
        while (i < array.length) : (i += 1) {
            if (i > 0) writer.writeAll(" ") catch {};
            if (elems[i]) |str_obj| writeStringObj(str_obj, writer);
        }
    }
    writer.writeAll("\n") catch {};
}

/// console.log / console.info / console.debug — join args with a space, print to stdout + newline.
export fn zeus_Console_log(this_ptr: ?*anyopaque, return_buffer_ptr: ?*anyopaque, args_ptr_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    _ = return_buffer_ptr;
    printArgs(args_ptr_ptr, std.io.getStdOut().writer());
}

export fn zeus_Console_info(this_ptr: ?*anyopaque, return_buffer_ptr: ?*anyopaque, args_ptr_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    _ = return_buffer_ptr;
    printArgs(args_ptr_ptr, std.io.getStdOut().writer());
}

export fn zeus_Console_debug(this_ptr: ?*anyopaque, return_buffer_ptr: ?*anyopaque, args_ptr_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    _ = return_buffer_ptr;
    printArgs(args_ptr_ptr, std.io.getStdOut().writer());
}

/// console.warn / console.error — same, to stderr.
export fn zeus_Console_warn(this_ptr: ?*anyopaque, return_buffer_ptr: ?*anyopaque, args_ptr_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    _ = return_buffer_ptr;
    printArgs(args_ptr_ptr, std.io.getStdErr().writer());
}

export fn zeus_Console_error(this_ptr: ?*anyopaque, return_buffer_ptr: ?*anyopaque, args_ptr_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    _ = return_buffer_ptr;
    printArgs(args_ptr_ptr, std.io.getStdErr().writer());
}
