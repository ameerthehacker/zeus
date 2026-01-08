const std = @import("std");
const builtin = @import("builtin");

// In release builds, debug logging is completely compiled out
const IS_DEBUG_BUILD = builtin.mode == .Debug;

// Cached debug flag - computed once at startup (only used in debug builds)
var debug_enabled_cached: ?bool = null;

fn isDebugEnabled() bool {
    // In release builds, always return false - compiler will eliminate dead code
    if (!IS_DEBUG_BUILD) return false;

    if (debug_enabled_cached) |cached| {
        return cached;
    }
    // First call - check environment variable
    const result = blk: {
        const env_val = std.posix.getenv("ZEUS_GC_DEBUG") orelse break :blk false;
        break :blk std.mem.eql(u8, env_val, "true");
    };
    debug_enabled_cached = result;
    return result;
}

pub inline fn log(allocator: std.mem.Allocator, prefix: []const u8, comptime fmt: []const u8, args: anytype) void {
    _ = allocator; // No longer needed
    // In release builds, this entire function compiles to nothing
    if (!IS_DEBUG_BUILD) return;
    if (!isDebugEnabled()) return;
    std.io.getStdOut().writer().print("zeus_gc: {s}: " ++ fmt ++ "\n", .{prefix} ++ args) catch {};
}
