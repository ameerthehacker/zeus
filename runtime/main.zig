const std = @import("std");

var gpa = std.heap.GeneralPurposeAllocator(.{}){};
const allocator = gpa.allocator();

fn isDebugEnabled() bool {
    const env_val = std.process.getEnvVarOwned(allocator, "ZEUS_GC_DEBUG") catch return false;
    defer allocator.free(env_val);
    return std.mem.eql(u8, env_val, "true");
}

fn log(comptime fmt: []const u8, args: anytype) void {
    if (!isDebugEnabled()) return;
    std.debug.print("zeus_gc: " ++ fmt ++ "\n", args);
}

export fn gc_alloc(size: u32) ?*anyopaque {
    const bytes = allocator.alloc(u8, size) catch return null;
    log("gc_alloc: {} bytes", .{size});
    return bytes.ptr;
}
