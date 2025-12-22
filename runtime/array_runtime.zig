// Zeus Array Runtime Functions
// Array primordial runtime functions for Zeus arrays

const std = @import("std");
const abi = @import("abi.zig");
const debug = @import("debug.zig");
const runtime_util = @import("runtime_util.zig");

// Forward declaration - will be resolved at link time
extern fn zeus_gc_alloc(size: u32) ?*anyopaque;

var array_gpa = std.heap.GeneralPurposeAllocator(.{}){};
const allocator = array_gpa.allocator();

// Constructor: zeus_array_constructor(this_ptr, return_buffer)
export fn zeus_array_constructor(this_ptr: *anyopaque, return_buffer_ptr: ?*anyopaque, initial_capacity_ptr: *anyopaque, element_size_ptr: *anyopaque) callconv(.C) void {
    _ = return_buffer_ptr; // void return, not used

    const array_ptr = @as(*abi.ZeusArrayObj, @ptrCast(@alignCast(this_ptr)));
    const _initial_capacity_ptr = @as(*u32, @ptrCast(@alignCast(initial_capacity_ptr)));
    const _element_size_ptr = @as(*u32, @ptrCast(@alignCast(element_size_ptr)));
    const initial_capacity = _initial_capacity_ptr.*;
    const element_size = _element_size_ptr.*;

    array_ptr.capacity = initial_capacity;
    array_ptr.length = 0;
    array_ptr.element_size = element_size;

    // Allocate initial data array (capacity * 1 byte for u8 elements)
    const data_size = @as(u32, @intCast(array_ptr.capacity * array_ptr.element_size));

    if (data_size != 0) {
        array_ptr.data = zeus_gc_alloc(data_size);
    }
}

// Push: zeus_array_push(this_ptr, return_buffer, value_ptr)
export fn zeus_array_push(this_ptr: *anyopaque, return_buffer_ptr: ?*anyopaque, value_ptr: *anyopaque) callconv(.C) void {
    _ = return_buffer_ptr; // void return, not used

    const array_ptr = @as(*abi.ZeusArrayObj, @ptrCast(@alignCast(this_ptr)));

    // Check if we need to resize
    if (array_ptr.length >= array_ptr.capacity) {
        // Double the capacity
        const new_capacity = array_ptr.capacity * 2;
        const new_data_size = @as(u32, @intCast(new_capacity * array_ptr.element_size));
        const new_data = zeus_gc_alloc(new_data_size);

        if (new_data == null) {
            debug.log(allocator, "array_push", "failed to allocate memory for resize", .{});
            return;
        }

        // Copy existing data (all bytes)
        if (array_ptr.data != null and array_ptr.length > 0) {
            const old_data_bytes = @as([*]u8, @ptrCast(array_ptr.data.?));
            const new_data_bytes = @as([*]u8, @ptrCast(new_data.?));
            const copy_size = @as(usize, @intCast(array_ptr.length * array_ptr.element_size));
            @memcpy(new_data_bytes[0..copy_size], old_data_bytes[0..copy_size]);
        }

        array_ptr.data = new_data;
        array_ptr.capacity = new_capacity;

        debug.log(allocator, "array_push", "resized array to capacity={}", .{new_capacity});
    }

    // Add the new element at the correct offset
    if (array_ptr.data != null) {
        const data_bytes = @as([*]u8, @ptrCast(@alignCast(array_ptr.data.?)));
        const value_bytes = @as([*]u8, @ptrCast(@alignCast(value_ptr)));

        // Calculate offset for the new element
        const offset = @as(usize, @intCast(array_ptr.length * array_ptr.element_size));
        const element_size = @as(usize, @intCast(array_ptr.element_size));

        // Copy element_size bytes from value_ptr to the correct position
        @memcpy(data_bytes[offset .. offset + element_size], value_bytes[0..element_size]);

        array_ptr.length += 1;
    }
}

// Pop: zeus_array_pop(this_ptr, return_buffer_ptr_ptr, [no params])
export fn zeus_array_pop(this_ptr: *anyopaque, return_buffer_ptr_ptr: ?*anyopaque) callconv(.C) void {
    const array_ptr = @as(*abi.ZeusArrayObj, @ptrCast(@alignCast(this_ptr)));
    const element_size = @as(u32, @intCast(array_ptr.element_size));

    if (array_ptr.length == 0 or array_ptr.data == null) {
        // Return zero/default value for empty array
        _ = runtime_util.allocateZeroedReturnBuffer(return_buffer_ptr_ptr, element_size);
        debug.log(allocator, "array_pop", "attempted to pop from empty array", .{});
        return;
    }

    // Get the last element
    const data_bytes = @as([*]u8, @ptrCast(@alignCast(array_ptr.data.?)));
    const last_offset = @as(usize, @intCast((array_ptr.length - 1) * array_ptr.element_size));

    // Decrease length
    array_ptr.length -= 1;

    // Allocate return buffer and copy the popped value
    if (runtime_util.allocateReturnBuffer(return_buffer_ptr_ptr, element_size)) |result_bytes| {
        @memcpy(result_bytes, data_bytes[last_offset .. last_offset + element_size]);
    }
}

// Get: zeus_array_get(this_ptr, return_buffer_ptr_ptr, index_ptr)
export fn zeus_array_get(this_ptr: *anyopaque, return_buffer_ptr_ptr: ?*anyopaque, index_ptr: *anyopaque) callconv(.C) void {
    const array_ptr = @as(*abi.ZeusArrayObj, @ptrCast(@alignCast(this_ptr)));
    const index_val_ptr = @as(*i32, @ptrCast(@alignCast(index_ptr)));
    const index = index_val_ptr.*;
    const element_size = @as(u32, @intCast(array_ptr.element_size));

    // Bounds checking
    if (index < 0 or index >= array_ptr.length or array_ptr.data == null) {
        // Return zero/default value for out of bounds
        _ = runtime_util.allocateZeroedReturnBuffer(return_buffer_ptr_ptr, element_size);
        debug.log(allocator, "array_get", "index {} out of bounds for length {}", .{ index, array_ptr.length });
        return;
    }

    // Get the element at the correct offset
    const data_bytes = @as([*]u8, @ptrCast(@alignCast(array_ptr.data.?)));
    const element_offset = @as(usize, @intCast(index)) * element_size;

    // Allocate return buffer and copy the element value
    if (runtime_util.allocateReturnBuffer(return_buffer_ptr_ptr, element_size)) |result_bytes| {
        @memcpy(result_bytes, data_bytes[element_offset .. element_offset + element_size]);
    }
}
