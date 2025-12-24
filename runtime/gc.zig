const std = @import("std");
const debug = @import("debug.zig");
const abi = @import("abi.zig");
const array_runtime = @import("array_runtime.zig");

// Import types from ABI module
const ZeusObjectHeader = abi.ZeusObjectHeader;
const ZeusObj = abi.ZeusObj;

// Structure to track allocated objects
const AllocatedObject = struct { ptr: *ZeusObj, size: u32, marked: bool };

pub const GC = struct {
    allocator: std.mem.Allocator,
    gc_roots: std.ArrayList(*ZeusObj),
    allocated_objects: std.ArrayList(AllocatedObject),
    alloc_mutex: std.Thread.Mutex,

    pub fn init(allocator: std.mem.Allocator) GC {
        return GC{
            .allocator = allocator,
            .gc_roots = std.ArrayList(*ZeusObj).init(allocator),
            .allocated_objects = std.ArrayList(AllocatedObject).init(allocator),
            .alloc_mutex = std.Thread.Mutex{},
        };
    }

    pub fn deinit(self: *GC) void {
        self.gc_roots.deinit();
        self.allocated_objects.deinit();
    }

    pub fn registerRoots(self: *GC, root_ptrs: []const *anyopaque) void {
        debug.log(self.allocator, "gc_register_roots", "registering {} roots", .{root_ptrs.len});

        // Add all pointers to our GC root set
        for (root_ptrs) |root_ptr| {
            const ptr_addr = @intFromPtr(root_ptr);
            // Safely cast the aligned pointer
            const casted_root_ptr = @as(*ZeusObj, @ptrFromInt(ptr_addr));
            self.gc_roots.append(casted_root_ptr) catch |err| {
                debug.log(self.allocator, "gc_register_roots", "failed to add root 0x{X}: {}", .{ ptr_addr, err });
            };
        }
    }

    pub fn gc(self: *GC) void {
        debug.log(self.allocator, "gc_cycle", "starting", .{});
        debug.log(self.allocator, "gc_cycle", "tracked roots: {}, allocated objects: {}", .{ self.gc_roots.items.len, self.allocated_objects.items.len });

        self.alloc_mutex.lock();
        defer self.alloc_mutex.unlock();

        // Clear all marks before marking phase
        for (self.allocated_objects.items) |*obj| {
            obj.marked = false;
        }

        // Mark phase: mark all reachable objects starting from GC roots
        for (self.gc_roots.items) |root| {
            self.markObject(root);
        }

        // Sweep phase: free all unmarked and unprotected objects
        self.sweep();

        debug.log(self.allocator, "gc_cycle", "complete, {} objects remaining", .{self.allocated_objects.items.len});
    }

    fn markObject(self: *GC, ptr: *ZeusObj) void {
        for (self.allocated_objects.items) |*obj| {
            const obj_addr = @intFromPtr(obj.ptr);
            const obj_end = obj_addr + obj.size;
            const ptr_addr = @intFromPtr(ptr);

            // Check if the pointer points into this allocated object
            if (ptr_addr >= obj_addr and ptr_addr < obj_end) {
                if (!obj.marked) {
                    obj.marked = true;
                    debug.log(self.allocator, "gc_mark", "marked object at 0x{X} (size={})", .{ obj_addr, obj.size });

                    // total nested objects to mark
                    const gc_offsets = obj.ptr.obj_header.getGcOffsets();
                    debug.log(self.allocator, "gc_mark", "marking {} nested objects", .{gc_offsets.len});

                    for (gc_offsets) |offset_u8| {
                        const offset = @as(usize, offset_u8);
                        const offset_addr = obj_addr + offset;
                        const nested_obj_addr = @as(**ZeusObj, @ptrFromInt(offset_addr));
                        const nested_obj = nested_obj_addr.*;
                        debug.log(self.allocator, "gc_mark", "marking nested object at 0x{X} (offset={})", .{ @intFromPtr(nested_obj), offset });
                        self.markObject(nested_obj);
                    }

                    const object_type_info = obj.ptr.obj_header.getObjectTypeInfo();

                    if (object_type_info.object_type == abi.ZeusObjectType.array and object_type_info.array_element_type == abi.ZeusType.object) {
                        const array_obj = @as(*abi.ZeusArrayObj, @ptrCast(obj.ptr));
                        if (array_obj.data) |data| {
                            const elements = @as([*]?*ZeusObj, @ptrCast(@alignCast(data)));
                            for (elements[0..array_obj.length], 0..) |element, index| {
                                if (element) |elem| {
                                    debug.log(self.allocator, "gc_mark", "marking array 0x{X}[{}]=0x{X}", .{ @intFromPtr(array_obj), index, @intFromPtr(elem) });
                                    self.markObject(elem);
                                }
                            }
                        }
                    }
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
                debug.log(self.allocator, "gc_sweep", "freeing object at 0x{X} (size={})", .{ @intFromPtr(obj.ptr), obj.size });

                // Cleanup array data if this is an array object
                // Array data is managed by array_runtime's own allocator
                if (obj.ptr.obj_header.getObjectTypeInfo().object_type == abi.ZeusObjectType.array) {
                    array_runtime.zeus_array_cleanup(obj.ptr);
                }

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

        debug.log(self.allocator, "gc_sweep", "freed {} objects ({} bytes)", .{ freed_count, freed_bytes });
    }

    pub fn alloc(self: *GC, size: u32) ?*anyopaque {
        const bytes = self.allocator.alloc(u8, size) catch return null;

        // Track the allocated object
        self.alloc_mutex.lock();
        defer self.alloc_mutex.unlock();

        const casted_ptr = @as(*ZeusObj, @ptrFromInt(@intFromPtr(bytes.ptr)));
        self.allocated_objects.append(AllocatedObject{ .ptr = casted_ptr, .size = size, .marked = false }) catch |err| {
            debug.log(self.allocator, "gc_alloc", "failed to track allocation: {}", .{err});
            // Still return the allocation even if tracking fails
        };

        debug.log(self.allocator, "gc_alloc", "{} bytes at 0x{X}, total objects: {}", .{ size, @intFromPtr(bytes.ptr), self.allocated_objects.items.len });
        return bytes.ptr;
    }

    pub fn clearRoots(self: *GC) void {
        self.gc_roots.clearRetainingCapacity();
    }
};
