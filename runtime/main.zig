// Zeus Garbage Collector Runtime with libunwind-based stack walking
//
// To compile this runtime on macOS, make sure to link with libunwind:
// zig build-lib runtime/main.zig -lc -lunwind -target aarch64-macos
// or add to your build.zig: lib.linkSystemLibrary("unwind");
//
// libunwind provides robust stack traversal that handles:
// - Optimized code with frame pointer omission
// - Different calling conventions
// - Exception handling frames
// - Signal handler frames
// - More accurate unwinding than manual frame walking

const std = @import("std");
const stackmap = @import("stackmap.zig");
const gc_mod = @import("gc.zig");
const debug = @import("debug.zig");

var gpa = std.heap.GeneralPurposeAllocator(.{}){};
const allocator = gpa.allocator();

// Global GC instance
var gc_instance = gc_mod.GC.init(allocator);

export fn gc_safepoint_slow_path() void {
    debug.log(allocator, "===GC START===", .{});

    // Clear previous GC roots
    gc_instance.clearRoots();

    // Walk the entire stack and get all GC root pointers
    const gc_root_pointers = stackmap.walkStackAndProcessRoots(allocator) catch |err| {
        debug.log(allocator, "stack_walk: failed to collect GC roots: {}", .{err});
        return;
    };
    defer gc_root_pointers.deinit();

    // Register all found pointers as GC roots
    gc_instance.registerRoots(gc_root_pointers.items);

    // TODO: Trigger GC based on good heuristics
    gc_instance.gc();
    debug.log(allocator, "===GC END===", .{});
}

export fn gc_alloc(size: u32) ?*anyopaque {
    // Use the GC instance to allocate and track the object
    return gc_instance.alloc(size);
}
