const std = @import("std");

pub fn isDebugEnabled(allocator: std.mem.Allocator) bool {
    const env_val = std.process.getEnvVarOwned(allocator, "ZEUS_GC_DEBUG") catch return false;
    defer allocator.free(env_val);
    return std.mem.eql(u8, env_val, "true");
}

pub fn log(allocator: std.mem.Allocator, comptime fmt: []const u8, args: anytype) void {
    if (!isDebugEnabled(allocator)) return;
    std.io.getStdOut().writer().print("zeus_gc: " ++ fmt ++ "\n", args) catch {};
} 