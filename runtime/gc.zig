const std = @import("std");
const debug = @import("debug.zig");

// Structure to track GC roots - just the pointer and whether it's marked
const GCRoot = struct { ptr: *anyopaque, marked: bool };

// Structure to track allocated objects
const AllocatedObject = struct { ptr: *anyopaque, size: u32, marked: bool };

pub const GC = struct {
    allocator: std.mem.Allocator,
    gc_roots: std.ArrayList(GCRoot),
    allocated_objects: std.ArrayList(AllocatedObject),
    alloc_mutex: std.Thread.Mutex,

    pub fn init(allocator: std.mem.Allocator) GC {
        return GC{
            .allocator = allocator,
            .gc_roots = std.ArrayList(GCRoot).init(allocator),
            .allocated_objects = std.ArrayList(AllocatedObject).init(allocator),
            .alloc_mutex = std.Thread.Mutex{},
        };
    }

    pub fn deinit(self: *GC) void {
        self.gc_roots.deinit();
        self.allocated_objects.deinit();
    }

    pub fn registerRoots(self: *GC, root_ptrs: []const *anyopaque) void {
        debug.log(self.allocator, "gc_root: registering {} pointers", .{root_ptrs.len});

        // Add all pointers to our GC root set
        for (root_ptrs) |root_ptr| {
            self.gc_roots.append(GCRoot{
                .ptr = root_ptr,
                .marked = false,
            }) catch |err| {
                debug.log(self.allocator, "gc_root: failed to add root 0x{X}: {}", .{ @intFromPtr(root_ptr), err });
            };
        }
    }

    pub fn gc(self: *GC) void {
        debug.log(self.allocator, "gc_cycle: starting", .{});
        debug.log(self.allocator, "gc_cycle: tracked roots: {}, allocated objects: {}", .{ self.gc_roots.items.len, self.allocated_objects.items.len });

        self.alloc_mutex.lock();
        defer self.alloc_mutex.unlock();

        // Clear all marks before marking phase
        for (self.allocated_objects.items) |*obj| {
            obj.marked = false;
        }

        // Mark phase: mark all reachable objects starting from GC roots
        for (self.gc_roots.items) |*root| {
            root.marked = false;
            self.markObject(root);
        }

        // Sweep phase: free all unmarked and unprotected objects
        self.sweep();

        debug.log(self.allocator, "gc_cycle: complete, {} objects remaining", .{self.allocated_objects.items.len});
    }

    fn markObject(self: *GC, root: *GCRoot) void {
        root.marked = true;
        debug.log(self.allocator, "gc_mark: root at 0x{X}", .{@intFromPtr(root.ptr)});

        // Find the corresponding allocated object and mark it
        self.markAllocatedObject(@intFromPtr(root.ptr));

        // TODO: traverse object references here to mark transitively reachable objects
    }

    fn markAllocatedObject(self: *GC, ptr_addr: usize) void {
        for (self.allocated_objects.items) |*obj| {
            const obj_addr = @intFromPtr(obj.ptr);
            const obj_end = obj_addr + obj.size;

            // Check if the pointer points into this allocated object
            if (ptr_addr >= obj_addr and ptr_addr < obj_end) {
                if (!obj.marked) {
                    obj.marked = true;
                    debug.log(self.allocator, "gc_mark: allocated object at 0x{X} (size={})", .{ obj_addr, obj.size });

                    // TODO: Scan this object for references to other objects
                    // For now, we only mark the directly referenced object
                }
                break;
            }
        }
    }

    fn sweep(self: *GC) void {
        var freed_count: u32 = 0;
        var freed_bytes: u32 = 0;
        var i: usize = 0;
        while (i < self.allocated_objects.items.len) {
            const obj = &self.allocated_objects.items[i];

            // Only free if object is not marked
            if (!obj.marked) {
                // Object is not reachable and not protected, free it
                debug.log(self.allocator, "gc_sweep: freeing object at 0x{X} (size={})", .{ @intFromPtr(obj.ptr), obj.size });

                // Create a slice from the pointer and size to free it properly
                const bytes = @as([*]u8, @ptrCast(obj.ptr))[0..obj.size];
                self.allocator.free(bytes);

                freed_count += 1;
                freed_bytes += obj.size;

                // Remove from tracking list by swapping with last element
                _ = self.allocated_objects.swapRemove(i);
                // Don't increment i since we swapped a new element to this position
            } else {
                i += 1;
            }
        }

        debug.log(self.allocator, "gc_sweep: freed {} objects ({} bytes)", .{ freed_count, freed_bytes });
    }

    pub fn alloc(self: *GC, size: u32) ?*anyopaque {
        const bytes = self.allocator.alloc(u8, size) catch return null;

        // Track the allocated object
        self.alloc_mutex.lock();
        defer self.alloc_mutex.unlock();

        self.allocated_objects.append(AllocatedObject{ 
            .ptr = bytes.ptr, 
            .size = size, 
            .marked = false 
        }) catch |err| {
            debug.log(self.allocator, "gc_alloc: failed to track allocation: {}", .{err});
            // Still return the allocation even if tracking fails
        };

        debug.log(self.allocator, "gc_alloc: {} bytes at 0x{X}, total objects: {}", .{ size, @intFromPtr(bytes.ptr), self.allocated_objects.items.len });
        return bytes.ptr;
    }

    pub fn clearRoots(self: *GC) void {
        self.gc_roots.clearRetainingCapacity();
    }
}; 