// Zeus C-FFI Runtime — the generic, library-agnostic support layer for pure-Zeus C bindings.
//
// Unlike the "fat ABI" runtime functions (zeus_Console_log, zeus_string_concat, ...) which take
// (this_ptr, return_buffer_ptr, ...arg_ptr_ptr), every helper here is a PLAIN direct C-ABI function
// (native by-value params/returns). They are bound on the Zeus side via extern("C","zeus_...") — the
// same direct path as raw libc symbols — because we own both ends. This layer is added ONCE and is
// never per-library: it just marshals between Zeus values (strings, u8[] buffers) and raw C memory.

const std = @import("std");
const abi = @import("abi.zig");
const runtime_util = @import("runtime_util.zig");
const string_runtime = @import("string_runtime.zig");
const gc_runtime = @import("gc_runtime_boehm.zig");
const builtin = @import("builtin");

// Codegen-generated factories (declared per-file; resolved at link time to the same symbols).
extern fn zeus_new_u8_array(capacity: i32) *anyopaque;
extern fn zeus_new_string(data: *anyopaque) *anyopaque;

// ============================================================================
// String <-> C string marshalling
// ============================================================================

/// Allocate a NUL-terminated C copy of a Zeus string in GC memory and return a raw pointer to it.
/// GC-allocated (not malloc) so it is reclaimed automatically; Boehm is non-moving + conservative,
/// so the pointer stays valid while it is reachable on the Zeus stack across the C call.
pub export fn zeus_cstr_from_string(str_ptr: ?*anyopaque) callconv(.C) ?*anyopaque {
    const s = str_ptr orelse return null;
    const str = string_runtime.castToStringObj(s);
    const len: usize = if (str.length > 0) @intCast(str.length) else 0;
    const buf = gc_runtime.zeus_gc_alloc(@intCast(len + 1)) orelse return null;
    const dst = @as([*]u8, @ptrCast(@alignCast(buf)));
    if (len > 0) {
        if (str.data) |arr| {
            if (arr.data) |d| {
                const src = @as([*]const u8, @ptrCast(@alignCast(d)));
                @memcpy(dst[0..len], src[0..len]);
            }
        }
    }
    dst[len] = 0;
    return buf;
}

/// Build a Zeus string from `len` raw bytes at `ptr` (not required to be NUL-terminated). Used for
/// binary reads (e.g. read(2) results). Returns a *ZeusStringObj.
pub export fn zeus_string_from_bytes(ptr: ?*anyopaque, len: i64) callconv(.C) ?*anyopaque {
    // Zeus arrays/strings are i32-length-bounded. A checked cast avoids the ReleaseFast UB of a raw
    // @intCast: an out-of-range or negative len yields an empty string rather than corrupting memory.
    // (fs.readFileSync guards MAX_FILE_BYTES up front, so this is a defensive backstop.)
    const n: i32 = std.math.cast(i32, len) orelse 0;
    if (n <= 0) return zeus_new_string(zeus_new_u8_array(0));
    const arr = zeus_new_u8_array(n); // capacity == length == n, zeroed data buffer
    const arr_obj = runtime_util.castToArrayObj(arr);
    if (arr_obj.data) |d| {
        if (ptr) |src_ptr| {
            const dst = @as([*]u8, @ptrCast(@alignCast(d)));
            const src = @as([*]const u8, @ptrCast(@alignCast(src_ptr)));
            @memcpy(dst[0..@intCast(n)], src[0..@intCast(n)]);
        }
    }
    return zeus_new_string(arr);
}

/// Build a Zeus string from a NUL-terminated C string.
pub export fn zeus_string_from_cstr(cstr: ?*anyopaque) callconv(.C) ?*anyopaque {
    const p = cstr orelse return zeus_string_from_bytes(null, 0);
    const s = @as([*:0]const u8, @ptrCast(p));
    const len = std.mem.len(s);
    return zeus_string_from_bytes(p, @intCast(len));
}

/// 1 if the raw pointer is NULL, else 0. Returns cint (not bool) to keep the C-ABI unambiguous.
export fn zeus_cstr_is_null(ptr: ?*anyopaque) callconv(.C) i32 {
    return if (ptr == null) 1 else 0;
}

// ============================================================================
// Raw (non-GC) memory — stable buffers for C to fill; caller must free.
// ============================================================================

export fn zeus_malloc(size: i64) callconv(.C) ?*anyopaque {
    if (size <= 0) return null;
    return std.c.malloc(@intCast(size));
}

export fn zeus_free(ptr: ?*anyopaque) callconv(.C) void {
    std.c.free(ptr);
}

/// Grow/shrink a cMalloc'd buffer (libc realloc). Returns null on failure (caller keeps old ptr).
pub export fn zeus_realloc(ptr: ?*anyopaque, size: i64) callconv(.C) ?*anyopaque {
    if (size <= 0) return null;
    return std.c.realloc(ptr, @intCast(size));
}

// ============================================================================
// u8[] buffer bridge — hand an existing Zeus byte array's stable data pointer to C.
// ============================================================================

export fn zeus_bytes_ptr(arr_ptr: ?*anyopaque) callconv(.C) ?*anyopaque {
    const a = arr_ptr orelse return null;
    return runtime_util.castToArrayObj(a).data;
}

export fn zeus_bytes_len(arr_ptr: ?*anyopaque) callconv(.C) i64 {
    const a = arr_ptr orelse return 0;
    return @intCast(runtime_util.castToArrayObj(a).length);
}

// ============================================================================
// Pointer read/write by byte offset — the generic C-struct-field mechanism.
// Zeus reads struct fields (e.g. struct stat) via compile-time per-target offset constants, so the
// compiler never has to model C struct layout. Unaligned (align(1)) to allow odd field offsets.
// ============================================================================

inline fn addrAt(base: ?*anyopaque, offset: i64) usize {
    return @intFromPtr(base) + @as(usize, @intCast(offset));
}

export fn zeus_ptr_read_i8(base: ?*anyopaque, offset: i64) callconv(.C) i32 {
    return @as(*align(1) i8, @ptrFromInt(addrAt(base, offset))).*;
}
export fn zeus_ptr_read_i16(base: ?*anyopaque, offset: i64) callconv(.C) i32 {
    return @as(*align(1) i16, @ptrFromInt(addrAt(base, offset))).*;
}
export fn zeus_ptr_read_i32(base: ?*anyopaque, offset: i64) callconv(.C) i32 {
    return @as(*align(1) i32, @ptrFromInt(addrAt(base, offset))).*;
}
export fn zeus_ptr_read_i64(base: ?*anyopaque, offset: i64) callconv(.C) i64 {
    return @as(*align(1) i64, @ptrFromInt(addrAt(base, offset))).*;
}
export fn zeus_ptr_read_u32(base: ?*anyopaque, offset: i64) callconv(.C) i64 {
    return @as(*align(1) u32, @ptrFromInt(addrAt(base, offset))).*;
}
export fn zeus_ptr_read_f64(base: ?*anyopaque, offset: i64) callconv(.C) f64 {
    return @as(*align(1) f64, @ptrFromInt(addrAt(base, offset))).*;
}
export fn zeus_ptr_read_ptr(base: ?*anyopaque, offset: i64) callconv(.C) ?*anyopaque {
    return @as(*align(1) ?*anyopaque, @ptrFromInt(addrAt(base, offset))).*;
}
/// Returns base + offset as a raw pointer (address arithmetic, NOT a dereference). Used to point at
/// an inline field within a C struct, e.g. the char[] d_name embedded in a struct dirent.
export fn zeus_ptr_offset(base: ?*anyopaque, offset: i64) callconv(.C) ?*anyopaque {
    return @ptrFromInt(addrAt(base, offset));
}
export fn zeus_ptr_write_i32(base: ?*anyopaque, offset: i64, value: i32) callconv(.C) void {
    @as(*align(1) i32, @ptrFromInt(addrAt(base, offset))).* = value;
}
export fn zeus_ptr_write_i64(base: ?*anyopaque, offset: i64, value: i64) callconv(.C) void {
    @as(*align(1) i64, @ptrFromInt(addrAt(base, offset))).* = value;
}

// ============================================================================
// errno + OS identity (avoids parsing struct utsname from Zeus)
// ============================================================================

export fn zeus_errno() callconv(.C) i32 {
    return @intCast(std.c._errno().*);
}

/// Reset errno to 0. Used before a call whose success/failure can only be told apart via errno
/// (e.g. readdir returning NULL for both end-of-stream and error).
export fn zeus_clear_errno() callconv(.C) void {
    std.c._errno().* = 0;
}

/// Node-style platform string.
pub export fn zeus_os_platform() callconv(.C) [*:0]const u8 {
    return switch (builtin.os.tag) {
        .macos => "darwin",
        .linux => "linux",
        else => "unknown",
    };
}

/// Node-style architecture string.
pub export fn zeus_os_arch() callconv(.C) [*:0]const u8 {
    return switch (builtin.cpu.arch) {
        .aarch64 => "arm64",
        .x86_64 => "x64",
        else => "unknown",
    };
}

/// Total physical memory in bytes (0 on failure/unsupported). The macOS/Linux split lives here in
/// Zig, not in Zeus. builtin.os.tag is comptime, so only the target's branch is compiled.
export fn zeus_os_totalmem() callconv(.C) i64 {
    switch (builtin.os.tag) {
        .macos => {
            var mem: u64 = 0;
            var len: usize = @sizeOf(u64);
            const rc = std.c.sysctlbyname("hw.memsize", &mem, &len, null, 0);
            if (rc != 0) return 0;
            return @intCast(mem);
        },
        .linux => {
            const pages = std.c.sysconf(std.c._SC.PHYS_PAGES);
            const page_size = std.c.sysconf(std.c._SC.PAGE_SIZE);
            if (pages <= 0 or page_size <= 0) return 0;
            return @as(i64, @intCast(pages)) * @as(i64, @intCast(page_size));
        },
        else => return 0,
    }
}

/// Free physical memory in bytes (0 on failure/unsupported). macOS counts free pages via sysctl;
/// Linux uses the available-pages sysconf (sysconf is Linux-only in this libc binding). Both are
/// best-effort snapshots.
export fn zeus_os_freemem() callconv(.C) i64 {
    switch (builtin.os.tag) {
        .macos => {
            var free_pages: u32 = 0;
            var len: usize = @sizeOf(u32);
            if (std.c.sysctlbyname("vm.page_free_count", &free_pages, &len, null, 0) != 0) return 0;
            var page_size: u32 = 0;
            len = @sizeOf(u32);
            if (std.c.sysctlbyname("hw.pagesize", &page_size, &len, null, 0) != 0) return 0;
            return @as(i64, @intCast(free_pages)) * @as(i64, @intCast(page_size));
        },
        .linux => {
            const pages = std.c.sysconf(std.c._SC.AVPHYS_PAGES);
            const page_size = std.c.sysconf(std.c._SC.PAGE_SIZE);
            if (pages <= 0 or page_size <= 0) return 0;
            return @as(i64, @intCast(pages)) * @as(i64, @intCast(page_size));
        },
        else => return 0,
    }
}

/// Number of logical CPUs (>= 1). os.type/release/version/machine are read directly from libc
/// uname() in @std/os; only freemem (sysctl null-pointer + sysconf split) and this cpu count stay
/// here, where the macOS/Linux difference is cleaner to express.
export fn zeus_os_cpu_count() callconv(.C) i64 {
    return @intCast(std.Thread.getCpuCount() catch 1);
}
