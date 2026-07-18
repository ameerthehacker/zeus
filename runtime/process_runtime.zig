// Zeus Process Runtime — backs the ambient global `process` object (a Process primordial class,
// see internal/prelude/process.zs). Each function is a fat-ABI method body forwarding to libc,
// exactly like the Console primordial. String marshalling reuses the c_ffi_runtime helpers.

const std = @import("std");
const runtime_util = @import("runtime_util.zig");
const string_runtime = @import("string_runtime.zig");
const c_ffi = @import("c_ffi_runtime.zig");

extern "c" fn getcwd(buf: [*]u8, size: usize) ?[*:0]u8;
extern "c" fn getenv(name: [*:0]const u8) ?[*:0]u8;
extern "c" fn setenv(name: [*:0]const u8, value: [*:0]const u8, overwrite: c_int) c_int;
extern "c" fn chdir(path: [*:0]const u8) c_int;
extern "c" fn getpid() c_int;
extern "c" fn exit(code: c_int) noreturn;

// Deref a fat-ABI object arg (ptr-to-object-pointer) to the object pointer.
inline fn argObj(ptr_ptr: *anyopaque) *anyopaque {
    return @as(**anyopaque, @ptrCast(@alignCast(ptr_ptr))).*;
}

// Write a Zeus string result through the return-buffer wrapper ABI.
fn writeString(return_buffer_ptr_ptr: ?*anyopaque, str: ?*anyopaque) void {
    if (runtime_util.allocateReturnBuffer(return_buffer_ptr_ptr, @sizeOf(*anyopaque))) |bytes| {
        const p = @as(**anyopaque, @ptrCast(@alignCast(bytes.ptr)));
        p.* = str orelse c_ffi.zeus_string_from_bytes(null, 0).?;
    }
}

fn writeBool(return_buffer_ptr_ptr: ?*anyopaque, value: bool) void {
    if (runtime_util.allocateReturnBuffer(return_buffer_ptr_ptr, @sizeOf(bool))) |bytes| {
        bytes[0] = if (value) 1 else 0;
    }
}

/// process.cwd(): string — retries with a growing heap buffer so deep paths (getcwd ERANGE) still
/// resolve instead of silently returning "".
export fn zeus_process_cwd(this_ptr: ?*anyopaque, return_buffer_ptr_ptr: ?*anyopaque) callconv(.C) void {
    _ = this_ptr;
    const cwd_max_buf: usize = 1 << 20; // 1 MiB — well past any real path length
    var size: usize = 4096;
    while (size <= cwd_max_buf) : (size *= 2) {
        const buf = std.c.malloc(size) orelse break;
        if (getcwd(@ptrCast(buf), size)) |r| {
            const str = c_ffi.zeus_string_from_cstr(@ptrCast(r));
            std.c.free(buf);
            writeString(return_buffer_ptr_ptr, str);
            return;
        }
        std.c.free(buf); // getcwd failed (ERANGE => grow; other errno => next iters just stop at the cap)
    }
    writeString(return_buffer_ptr_ptr, c_ffi.zeus_string_from_bytes(null, 0));
}

/// process.getEnv(name: string): string — "" if unset.
export fn zeus_process_getenv(this_ptr: ?*anyopaque, return_buffer_ptr_ptr: ?*anyopaque, name_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    const cname = c_ffi.zeus_cstr_from_string(argObj(name_ptr));
    var str: ?*anyopaque = null;
    if (cname) |cn| {
        if (getenv(@ptrCast(cn))) |val| {
            str = c_ffi.zeus_string_from_cstr(@ptrCast(val));
        }
    }
    if (str == null) str = c_ffi.zeus_string_from_bytes(null, 0);
    writeString(return_buffer_ptr_ptr, str);
}

/// process.hasEnv(name: string): boolean
export fn zeus_process_hasenv(this_ptr: ?*anyopaque, return_buffer_ptr_ptr: ?*anyopaque, name_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    var present: bool = false;
    if (c_ffi.zeus_cstr_from_string(argObj(name_ptr))) |cn| {
        present = getenv(@ptrCast(cn)) != null;
    }
    writeBool(return_buffer_ptr_ptr, present);
}

/// process.setEnv(name: string, value: string): void
export fn zeus_process_setenv(this_ptr: ?*anyopaque, return_buffer_ptr: ?*anyopaque, name_ptr: *anyopaque, value_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    _ = return_buffer_ptr;
    const cname = c_ffi.zeus_cstr_from_string(argObj(name_ptr));
    const cval = c_ffi.zeus_cstr_from_string(argObj(value_ptr));
    if (cname) |cn| {
        if (cval) |cv| {
            _ = setenv(@ptrCast(cn), @ptrCast(cv), 1);
        }
    }
}

/// process.chdir(path: string): boolean
export fn zeus_process_chdir(this_ptr: ?*anyopaque, return_buffer_ptr_ptr: ?*anyopaque, path_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    var ok: bool = false;
    if (c_ffi.zeus_cstr_from_string(argObj(path_ptr))) |cp| {
        ok = chdir(@ptrCast(cp)) == 0;
    }
    writeBool(return_buffer_ptr_ptr, ok);
}

/// process.pid(): i32
export fn zeus_process_pid(this_ptr: ?*anyopaque, return_buffer_ptr_ptr: ?*anyopaque) callconv(.C) void {
    _ = this_ptr;
    if (runtime_util.allocateReturnBuffer(return_buffer_ptr_ptr, @sizeOf(i32))) |bytes| {
        const p = @as(*i32, @ptrCast(@alignCast(bytes.ptr)));
        p.* = @intCast(getpid());
    }
}

/// process.exit(code: i32): void
export fn zeus_process_exit(this_ptr: ?*anyopaque, return_buffer_ptr: ?*anyopaque, code_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    _ = return_buffer_ptr;
    const code = @as(*i32, @ptrCast(@alignCast(code_ptr))).*;
    exit(@intCast(code));
}
