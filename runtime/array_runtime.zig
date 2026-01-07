// Zeus Array Runtime Functions
// Array primordial runtime functions for Zeus arrays

const std = @import("std");
const abi = @import("abi.zig");
const debug = @import("debug.zig");
const runtime_util = @import("runtime_util.zig");

var array_gpa = std.heap.GeneralPurposeAllocator(.{}){};
const allocator = array_gpa.allocator();

// Constants
const ARRAY_GROWTH_FACTOR: u32 = 2;
const ARRAY_MIN_CAPACITY: u32 = 4;

// ============================================================================
// Helper Functions
// ============================================================================

/// Safely cast anyopaque pointer to ZeusArrayObj pointer
const castToArrayObj = runtime_util.castToArrayObj;

/// Get the size of each element in the array
inline fn getElementSize(array_ptr: *abi.ZeusArrayObj) u32 {
    const type_info = array_ptr.obj_header.getObjectTypeInfo();
    return runtime_util.getZeusTypeSize(type_info.array_element_type);
}

/// Get byte pointer to array data
inline fn getDataBytes(array_ptr: *abi.ZeusArrayObj) ?[*]u8 {
    if (array_ptr.data) |data| {
        return @as([*]u8, @ptrCast(@alignCast(data)));
    }
    return null;
}

/// Calculate byte offset for an element at given index
inline fn getElementOffset(index: u32, element_size: u32) usize {
    return @as(usize, @intCast(index)) * element_size;
}

/// Write default value for a Zeus type at the given memory location
fn writeDefaultValue(dest: [*]u8, zeus_type: abi.ZeusType, element_size: u32) void {
    switch (zeus_type) {
        ._i8 => {
            const ptr = @as(*i8, @ptrCast(@alignCast(dest)));
            ptr.* = 0;
        },
        ._i16 => {
            const ptr = @as(*i16, @ptrCast(@alignCast(dest)));
            ptr.* = 0;
        },
        ._i32 => {
            const ptr = @as(*i32, @ptrCast(@alignCast(dest)));
            ptr.* = 0;
        },
        ._i64 => {
            const ptr = @as(*i64, @ptrCast(@alignCast(dest)));
            ptr.* = 0;
        },
        ._f32 => {
            const ptr = @as(*f32, @ptrCast(@alignCast(dest)));
            ptr.* = 0.0;
        },
        ._f64 => {
            const ptr = @as(*f64, @ptrCast(@alignCast(dest)));
            ptr.* = 0.0;
        },
        ._bool => {
            const ptr = @as(*bool, @ptrCast(@alignCast(dest)));
            ptr.* = false;
        },
        .object => {
            const ptr = @as(*?*anyopaque, @ptrCast(@alignCast(dest)));
            ptr.* = null;
        },
        ._null => {
            // For null type, just zero out the memory
            @memset(dest[0..element_size], 0);
        },
    }
}

/// Initialize a range of elements with default values
fn initializeWithDefaults(array_ptr: *abi.ZeusArrayObj, start_index: u32, end_index: u32) void {
    if (start_index >= end_index) return;
    if (getDataBytes(array_ptr)) |data_bytes| {
        const element_size = getElementSize(array_ptr);
        const type_info = array_ptr.obj_header.getObjectTypeInfo();
        const element_type = type_info.array_element_type;

        var i = start_index;
        while (i < end_index) : (i += 1) {
            const offset = getElementOffset(i, element_size);
            writeDefaultValue(data_bytes + offset, element_type, element_size);
        }
    }
}

/// Allocate a new data buffer with the specified capacity
fn allocateDataBuffer(capacity: u32, element_size: u32) ?*anyopaque {
    if (capacity == 0 or element_size == 0) {
        return null;
    }
    const data_size = capacity * element_size;
    return runtime_util.allocateRawBytes(allocator, data_size);
}

/// Free the array's data buffer and reset array fields
fn freeDataBuffer(array_ptr: *abi.ZeusArrayObj) void {
    if (array_ptr.data != null and array_ptr.capacity > 0) {
        const element_size = getElementSize(array_ptr);
        const data_size = @as(usize, @intCast(array_ptr.capacity * element_size));
        const data_bytes = @as([*]u8, @ptrCast(array_ptr.data.?))[0..data_size];
        allocator.free(data_bytes);

        debug.log(allocator, "array_cleanup", "freed array data buffer of {} bytes", .{data_size});

        array_ptr.data = null;
        array_ptr.length = 0;
        array_ptr.capacity = 0;
    }
}

/// Resize array to new capacity, copying existing data and initializing new slots with defaults
fn resizeArray(array_ptr: *abi.ZeusArrayObj, new_capacity: u32) bool {
    const element_size = getElementSize(array_ptr);
    const new_data = allocateDataBuffer(new_capacity, element_size);

    if (new_data == null) {
        debug.log(allocator, "array_resize", "failed to allocate memory for resize", .{});
        return false;
    }

    const old_length = array_ptr.length;

    // Copy existing data to new buffer
    if (array_ptr.data != null and array_ptr.length > 0) {
        const old_data_bytes = getDataBytes(array_ptr).?;
        const new_data_bytes = @as([*]u8, @ptrCast(new_data.?));
        const copy_size = getElementOffset(array_ptr.length, element_size);
        @memcpy(new_data_bytes[0..copy_size], old_data_bytes[0..copy_size]);
    }

    // Free old buffer before replacing
    if (array_ptr.data != null and array_ptr.capacity > 0) {
        const old_data_size = @as(usize, @intCast(array_ptr.capacity * element_size));
        const old_data_bytes = @as([*]u8, @ptrCast(array_ptr.data.?))[0..old_data_size];
        allocator.free(old_data_bytes);
    }

    array_ptr.data = new_data;
    array_ptr.capacity = new_capacity;

    // Initialize new capacity slots with default values
    initializeWithDefaults(array_ptr, old_length, new_capacity);

    debug.log(allocator, "array_resize", "resized array to capacity={}, initialized slots {}..{}", .{ new_capacity, old_length, new_capacity });
    return true;
}

// ============================================================================
// Exported Runtime Functions
// ============================================================================

/// Cleanup function called by GC when an array object is being freed
/// Frees the array's data buffer using the array allocator
pub fn zeus_array_cleanup(array_obj_ptr: *anyopaque) callconv(.C) void {
    const array_ptr = castToArrayObj(array_obj_ptr);
    freeDataBuffer(array_ptr);
}

/// Constructor: zeus_array_constructor(this_ptr, return_buffer, initial_capacity_ptr)
export fn zeus_array_constructor(this_ptr: *anyopaque, return_buffer_ptr: ?*anyopaque, initial_capacity_ptr: *anyopaque) callconv(.C) void {
    _ = return_buffer_ptr; // void return, not used

    const array_ptr = castToArrayObj(this_ptr);
    const capacity_ptr = @as(*u32, @ptrCast(@alignCast(initial_capacity_ptr)));
    const initial_capacity = capacity_ptr.*;

    array_ptr.length = 0;
    array_ptr.capacity = initial_capacity;

    // Allocate initial data buffer
    const element_size = getElementSize(array_ptr);
    array_ptr.data = allocateDataBuffer(initial_capacity, element_size);

    if (initial_capacity > 0 and array_ptr.data == null) {
        debug.log(allocator, "array_constructor", "failed to allocate memory for array data", .{});
        array_ptr.capacity = 0;
    } else if (initial_capacity > 0) {
        // Initialize all capacity with default values
        initializeWithDefaults(array_ptr, 0, initial_capacity);
    }
}

/// Push: zeus_array_push(this_ptr, return_buffer, value_ptr)
export fn zeus_array_push(this_ptr: *anyopaque, return_buffer_ptr: ?*anyopaque, value_ptr: *anyopaque) callconv(.C) void {
    _ = return_buffer_ptr; // void return, not used

    const array_ptr = castToArrayObj(this_ptr);
    const element_size = getElementSize(array_ptr);

    // Check if we need to resize
    if (array_ptr.length >= array_ptr.capacity) {
        // Calculate new capacity: use minimum capacity if currently 0, otherwise double
        const new_capacity = if (array_ptr.capacity == 0)
            ARRAY_MIN_CAPACITY
        else
            array_ptr.capacity * ARRAY_GROWTH_FACTOR;

        if (!resizeArray(array_ptr, new_capacity)) {
            return; // Resize failed, error already logged
        }
    }

    // Add the new element at the correct offset
    if (getDataBytes(array_ptr)) |data_bytes| {
        const value_bytes = @as([*]u8, @ptrCast(@alignCast(value_ptr)));
        const offset = getElementOffset(array_ptr.length, element_size);
        @memcpy(data_bytes[offset .. offset + element_size], value_bytes[0..element_size]);
        array_ptr.length += 1;
    }
}

/// Pop: zeus_array_pop(this_ptr, return_buffer_ptr_ptr)
export fn zeus_array_pop(this_ptr: *anyopaque, return_buffer_ptr_ptr: ?*anyopaque) callconv(.C) void {
    const array_ptr = castToArrayObj(this_ptr);
    const element_size = getElementSize(array_ptr);

    // Handle empty array case
    if (array_ptr.length == 0) {
        _ = runtime_util.allocateZeroedReturnBuffer(return_buffer_ptr_ptr, element_size);
        debug.log(allocator, "array_pop", "attempted to pop from empty array", .{});
        return;
    }

    // Get the last element
    if (getDataBytes(array_ptr)) |data_bytes| {
        const last_offset = getElementOffset(array_ptr.length - 1, element_size);
        array_ptr.length -= 1;

        // Allocate return buffer and copy the popped value
        if (runtime_util.allocateReturnBuffer(return_buffer_ptr_ptr, element_size)) |result_bytes| {
            @memcpy(result_bytes, data_bytes[last_offset .. last_offset + element_size]);
        }
    } else {
        _ = runtime_util.allocateZeroedReturnBuffer(return_buffer_ptr_ptr, element_size);
        debug.log(allocator, "array_pop", "array data is null", .{});
    }
}

/// Get: zeus_array_get(this_ptr, return_buffer_ptr_ptr, index_ptr)
export fn zeus_array_get(this_ptr: *anyopaque, return_buffer_ptr_ptr: ?*anyopaque, index_ptr: *anyopaque) callconv(.C) void {
    const array_ptr = castToArrayObj(this_ptr);
    const index_val_ptr = @as(*i32, @ptrCast(@alignCast(index_ptr)));
    const index = index_val_ptr.*;
    const element_size = getElementSize(array_ptr);

    // Bounds checking
    if (index < 0 or index >= array_ptr.length) {
        _ = runtime_util.allocateZeroedReturnBuffer(return_buffer_ptr_ptr, element_size);
        debug.log(allocator, "array_get", "index {} out of bounds for length {}", .{ index, array_ptr.length });
        return;
    }

    // Get the element at the correct offset
    if (getDataBytes(array_ptr)) |data_bytes| {
        const element_offset = getElementOffset(@as(u32, @intCast(index)), element_size);

        // Allocate return buffer and copy the element value
        if (runtime_util.allocateReturnBuffer(return_buffer_ptr_ptr, element_size)) |result_bytes| {
            @memcpy(result_bytes, data_bytes[element_offset .. element_offset + element_size]);
        }
    } else {
        _ = runtime_util.allocateZeroedReturnBuffer(return_buffer_ptr_ptr, element_size);
        debug.log(allocator, "array_get", "array data is null", .{});
    }
}

/// Set: zeus_array_set(this_ptr, return_buffer_ptr, index_ptr, value_ptr)
/// Sets value at the specified index, resizing array if necessary
export fn zeus_array_set(this_ptr: *anyopaque, return_buffer_ptr: ?*anyopaque, index_ptr: *anyopaque, value_ptr: *anyopaque) callconv(.C) void {
    _ = return_buffer_ptr; // void return, not used

    const array_ptr = castToArrayObj(this_ptr);
    const index_val_ptr = @as(*i32, @ptrCast(@alignCast(index_ptr)));
    const index = index_val_ptr.*;
    const element_size = getElementSize(array_ptr);

    // Validate index is non-negative
    if (index < 0) {
        debug.log(allocator, "array_set", "negative index {} not allowed", .{index});
        return;
    }

    const target_index = @as(u32, @intCast(index));

    // Check if we need to resize to accommodate this index
    if (target_index >= array_ptr.capacity) {
        // Calculate new capacity: ensure it's at least index + 1, but use growth factor for efficiency
        var new_capacity = if (array_ptr.capacity == 0) ARRAY_MIN_CAPACITY else array_ptr.capacity;

        while (new_capacity <= target_index) {
            new_capacity *= ARRAY_GROWTH_FACTOR;
        }

        if (!resizeArray(array_ptr, new_capacity)) {
            debug.log(allocator, "array_set", "failed to resize for index {}", .{target_index});
            return; // Resize failed, error already logged
        }
    }

    // Set the value at the specified index
    if (getDataBytes(array_ptr)) |data_bytes| {
        const value_bytes = @as([*]u8, @ptrCast(@alignCast(value_ptr)));
        const offset = getElementOffset(target_index, element_size);
        @memcpy(data_bytes[offset .. offset + element_size], value_bytes[0..element_size]);

        // Update length if we're setting beyond current length
        if (target_index >= array_ptr.length) {
            array_ptr.length = target_index + 1;
        }

        debug.log(allocator, "array_set", "set value at index {}, new length={}", .{ target_index, array_ptr.length });
    }
}
