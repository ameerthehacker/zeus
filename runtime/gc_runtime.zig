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

// Use C allocator for fast small allocations
const allocator = std.heap.c_allocator;

// Global GC instance
var gc_instance = gc_mod.GC.init(allocator);

// GC threshold - only run GC when we have this many allocations
const GC_ALLOCATION_THRESHOLD: u32 = 100;
var allocations_since_gc: u32 = 0;

export fn zeus_gc_poll() void {
    // Only run GC when we've accumulated enough allocations
    // This avoids expensive stack walking on every safepoint
    if (allocations_since_gc < GC_ALLOCATION_THRESHOLD) {
        return;
    }

    debug.log(allocator, "gc_poll", "===GC POLL START (after {} allocations)===", .{allocations_since_gc});
    allocations_since_gc = 0;

    // Clear previous GC roots
    gc_instance.clearRoots();

    // Walk the entire stack and get all GC root pointers
    const gc_roots = stackmap.walkStackAndProcessRoots(allocator) catch |err| {
        debug.log(allocator, "gc_stack_walk", "failed to collect GC roots: {}", .{err});
        return;
    };
    defer gc_roots.deinit();

    // Register all found pointers as GC roots
    gc_instance.registerRoots(gc_roots.items);

    gc_instance.gc();
    debug.log(allocator, "gc_poll", "===GC POLL END===", .{});
}

pub export fn zeus_gc_alloc(size: u32) ?*anyopaque {
    // Track allocations for GC threshold
    allocations_since_gc += 1;

    // Use the GC instance to allocate and track the object
    return gc_instance.alloc(size);
}
