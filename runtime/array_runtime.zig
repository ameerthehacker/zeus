// Zeus Array Runtime Functions
// Array primordial runtime functions for Zeus arrays

const std = @import("std");
const abi = @import("abi.zig");
const debug = @import("debug.zig");
const runtime_util = @import("runtime_util.zig");

var array_gpa = std.heap.GeneralPurposeAllocator(.{}){};
const allocator = array_gpa.allocator();

fn getElementSize(array_ptr: *abi.ZeusArrayObj) u32 {
    const type_info = array_ptr.obj_header.getObjectTypeInfo();
    return runtime_util.getZeusTypeSize(type_info.array_element_type);
}

/// Cleanup function called by GC when an array object is being freed
/// Frees the array's data buffer using the array allocator
pub fn zeus_array_cleanup(array_obj_ptr: *anyopaque) callconv(.C) void {
    const array_ptr = @as(*abi.ZeusArrayObj, @ptrCast(@alignCast(array_obj_ptr)));

    if (array_ptr.data != null and array_ptr.capacity > 0) {
        const element_size = getElementSize(array_ptr);
        const data_size = @as(usize, @intCast(array_ptr.capacity * element_size));

        // Reconstruct the slice to free it properly
        const data_bytes = @as([*]u8, @ptrCast(array_ptr.data.?))[0..data_size];
        allocator.free(data_bytes);

        debug.log(allocator, "array_cleanup", "freed array data buffer of {} bytes", .{data_size});

        array_ptr.data = null;
        array_ptr.length = 0;
        array_ptr.capacity = 0;
    }
}

// Constructor: zeus_array_constructor(this_ptr, return_buffer)
export fn zeus_array_constructor(this_ptr: *anyopaque, return_buffer_ptr: ?*anyopaque, initial_capacity_ptr: *anyopaque) callconv(.C) void {
    _ = return_buffer_ptr; // void return, not used

    const array_ptr = @as(*abi.ZeusArrayObj, @ptrCast(@alignCast(this_ptr)));
    const _initial_capacity_ptr = @as(*u32, @ptrCast(@alignCast(initial_capacity_ptr)));
    const initial_capacity = _initial_capacity_ptr.*;

    array_ptr.capacity = initial_capacity;
    array_ptr.length = 0;
    const element_size = getElementSize(array_ptr);

    // Allocate initial data array (capacity * 1 byte for u8 elements)
    const data_size = @as(u32, @intCast(array_ptr.capacity * element_size));

    array_ptr.data = runtime_util.allocateRawBytes(allocator, data_size);
    if (data_size != 0 and array_ptr.data == null) {
        debug.log(allocator, "array_constructor", "failed to allocate memory for array data", .{});
        return;
    }
}

// Push: zeus_array_push(this_ptr, return_buffer, value_ptr)
export fn zeus_array_push(this_ptr: *anyopaque, return_buffer_ptr: ?*anyopaque, value_ptr: *anyopaque) callconv(.C) void {
    _ = return_buffer_ptr; // void return, not used

    const array_ptr = @as(*abi.ZeusArrayObj, @ptrCast(@alignCast(this_ptr)));
    const element_size = getElementSize(array_ptr);

    // Check if we need to resize
    if (array_ptr.length >= array_ptr.capacity) {
        // Double the capacity
        const new_capacity = array_ptr.capacity * 2;
        const new_data_size = @as(u32, @intCast(new_capacity * element_size));
        const new_data = runtime_util.allocateRawBytes(allocator, new_data_size);

        if (new_data == null) {
            debug.log(allocator, "array_push", "failed to allocate memory for resize", .{});
            return;
        }

        // Copy existing data (all bytes)
        if (array_ptr.data != null and array_ptr.length > 0) {
            const old_data_bytes = @as([*]u8, @ptrCast(array_ptr.data.?));
            const new_data_bytes = @as([*]u8, @ptrCast(new_data.?));
            const copy_size = @as(usize, @intCast(array_ptr.length * element_size));
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
        const offset = @as(usize, @intCast(array_ptr.length * element_size));
        // Copy element_size bytes from value_ptr to the correct position
        @memcpy(data_bytes[offset .. offset + element_size], value_bytes[0..element_size]);

        array_ptr.length += 1;
    }
}

// Pop: zeus_array_pop(this_ptr, return_buffer_ptr_ptr, [no params])
export fn zeus_array_pop(this_ptr: *anyopaque, return_buffer_ptr_ptr: ?*anyopaque) callconv(.C) void {
    const array_ptr = @as(*abi.ZeusArrayObj, @ptrCast(@alignCast(this_ptr)));
    const element_size = getElementSize(array_ptr);

    if (array_ptr.length == 0 or array_ptr.data == null) {
        // Return zero/default value for empty array
        _ = runtime_util.allocateZeroedReturnBuffer(return_buffer_ptr_ptr, element_size);
        debug.log(allocator, "array_pop", "attempted to pop from empty array", .{});
        return;
    }

    // Get the last element
    const data_bytes = @as([*]u8, @ptrCast(@alignCast(array_ptr.data.?)));
    const last_offset = @as(usize, @intCast((array_ptr.length - 1) * element_size));

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
    const element_size = getElementSize(array_ptr);

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
