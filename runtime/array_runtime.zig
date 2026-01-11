// Zeus Array Runtime Functions
// Array primordial runtime functions for Zeus arrays

const std = @import("std");
const abi = @import("abi.zig");
const debug = @import("debug.zig");
const runtime_util = @import("runtime_util.zig");

// Use C allocator for fast small allocations
const allocator = std.heap.c_allocator;

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

/// Copy: zeus_array_copy(this_ptr, return_buffer_ptr, source_ptr_ptr)
/// Copies data from source array to this array (shallow copy)
/// Auto-resizes destination if needed using existing resize logic
export fn zeus_array_copy(this_ptr: *anyopaque, return_buffer_ptr: ?*anyopaque, source_ptr_ptr: *anyopaque) callconv(.C) void {
    _ = return_buffer_ptr; // void return, not used

    const dest_array = castToArrayObj(this_ptr);

    // Dereference to get the source array pointer (passed via alloca)
    const source_obj_ptr = @as(**anyopaque, @ptrCast(@alignCast(source_ptr_ptr))).*;
    const source_array = castToArrayObj(source_obj_ptr);

    const element_size = getElementSize(dest_array);
    const copy_length = source_array.length;

    // Auto-resize destination if needed (reuses existing resize logic)
    if (dest_array.capacity < copy_length) {
        if (!resizeArray(dest_array, copy_length)) {
            return;
        }
    }

    // Copy the data
    if (source_array.data != null and dest_array.data != null and copy_length > 0) {
        const src_bytes = getDataBytes(source_array).?;
        const dest_bytes = getDataBytes(dest_array).?;
        const copy_size = getElementOffset(copy_length, element_size);
        @memcpy(dest_bytes[0..copy_size], src_bytes[0..copy_size]);
    }

    // Update destination length
    dest_array.length = copy_length;
}

/// CopyRange: zeus_array_copyRange(this_ptr, return_buffer_ptr, source_ptr_ptr, src_offset_ptr, dest_offset_ptr, count_ptr)
/// Copies a range of elements from source array to this array at the specified offset
/// This is used by the lowered concat/slice operations
export fn zeus_array_copyRange(this_ptr: *anyopaque, return_buffer_ptr: ?*anyopaque, source_ptr_ptr: *anyopaque, src_offset_ptr: *anyopaque, dest_offset_ptr: *anyopaque, count_ptr: *anyopaque) callconv(.C) void {
    _ = return_buffer_ptr; // void return, not used

    const dest_array = castToArrayObj(this_ptr);

    // Dereference to get the source array pointer (passed via alloca)
    const source_obj_ptr = @as(**anyopaque, @ptrCast(@alignCast(source_ptr_ptr))).*;
    const source_array = castToArrayObj(source_obj_ptr);

    // Get offset and count values
    const src_offset_val = @as(*i32, @ptrCast(@alignCast(src_offset_ptr))).*;
    const dest_offset_val = @as(*i32, @ptrCast(@alignCast(dest_offset_ptr))).*;
    const count_val = @as(*i32, @ptrCast(@alignCast(count_ptr))).*;

    // Validate and convert to unsigned
    if (src_offset_val < 0 or dest_offset_val < 0 or count_val <= 0) {
        return;
    }

    const src_offset: u32 = @intCast(src_offset_val);
    const dest_offset: u32 = @intCast(dest_offset_val);
    const count: u32 = @intCast(count_val);

    const element_size = getElementSize(dest_array);

    // Bounds check on source
    if (src_offset + count > source_array.length) {
        debug.log(allocator, "array_copyRange", "source range {}..{} exceeds length {}", .{ src_offset, src_offset + count, source_array.length });
        return;
    }

    // Ensure destination has enough capacity
    const required_length = dest_offset + count;
    if (dest_array.capacity < required_length) {
        if (!resizeArray(dest_array, required_length)) {
            return;
        }
    }

    // Copy the data
    if (source_array.data != null and dest_array.data != null and count > 0) {
        const src_bytes = getDataBytes(source_array).?;
        const dest_bytes = getDataBytes(dest_array).?;
        const src_byte_offset = getElementOffset(src_offset, element_size);
        const dest_byte_offset = getElementOffset(dest_offset, element_size);
        const copy_size = getElementOffset(count, element_size);
        @memcpy(dest_bytes[dest_byte_offset .. dest_byte_offset + copy_size], src_bytes[src_byte_offset .. src_byte_offset + copy_size]);
    }

    // Update destination length if we extended it
    if (required_length > dest_array.length) {
        dest_array.length = required_length;
    }

    debug.log(allocator, "array_copyRange", "copied {} elements from src[{}..] to dest[{}..], new length={}", .{ count, src_offset, dest_offset, dest_array.length });
}

// ============================================================================
// Helper Functions for Element Comparison
// ============================================================================

/// Compare two elements at given memory locations
/// Returns true if they are equal, false otherwise
fn elementsEqual(ptr1: [*]const u8, ptr2: [*]const u8, element_size: u32, element_type: abi.ZeusType) bool {
    switch (element_type) {
        ._i8 => {
            const a = @as(*const i8, @ptrCast(@alignCast(ptr1))).*;
            const b = @as(*const i8, @ptrCast(@alignCast(ptr2))).*;
            return a == b;
        },
        ._i16 => {
            const a = @as(*const i16, @ptrCast(@alignCast(ptr1))).*;
            const b = @as(*const i16, @ptrCast(@alignCast(ptr2))).*;
            return a == b;
        },
        ._i32 => {
            const a = @as(*const i32, @ptrCast(@alignCast(ptr1))).*;
            const b = @as(*const i32, @ptrCast(@alignCast(ptr2))).*;
            return a == b;
        },
        ._i64 => {
            const a = @as(*const i64, @ptrCast(@alignCast(ptr1))).*;
            const b = @as(*const i64, @ptrCast(@alignCast(ptr2))).*;
            return a == b;
        },
        ._f32 => {
            const a = @as(*const f32, @ptrCast(@alignCast(ptr1))).*;
            const b = @as(*const f32, @ptrCast(@alignCast(ptr2))).*;
            return a == b;
        },
        ._f64 => {
            const a = @as(*const f64, @ptrCast(@alignCast(ptr1))).*;
            const b = @as(*const f64, @ptrCast(@alignCast(ptr2))).*;
            return a == b;
        },
        ._bool => {
            const a = @as(*const bool, @ptrCast(@alignCast(ptr1))).*;
            const b = @as(*const bool, @ptrCast(@alignCast(ptr2))).*;
            return a == b;
        },
        .object => {
            const a = @as(*const ?*anyopaque, @ptrCast(@alignCast(ptr1))).*;
            const b = @as(*const ?*anyopaque, @ptrCast(@alignCast(ptr2))).*;
            return a == b;
        },
        ._null => {
            // For null type, do byte comparison
            return std.mem.eql(u8, ptr1[0..element_size], ptr2[0..element_size]);
        },
    }
}

/// Copy raw data between arrays
fn copyArrayData(dest_array: *abi.ZeusArrayObj, src_array: *abi.ZeusArrayObj, dest_offset: u32, src_offset: u32, count: u32) void {
    if (count == 0) return;
    const element_size = getElementSize(dest_array);
    if (getDataBytes(dest_array)) |dest_bytes| {
        if (getDataBytes(src_array)) |src_bytes| {
            const dest_start = getElementOffset(dest_offset, element_size);
            const src_start = getElementOffset(src_offset, element_size);
            const copy_size = getElementOffset(count, element_size);
            @memcpy(dest_bytes[dest_start .. dest_start + copy_size], src_bytes[src_start .. src_start + copy_size]);
        }
    }
}

// ============================================================================
// New Array Methods
// ============================================================================

// Note: concat and slice are handled entirely at the IR lowering level
// They use NEW_OBJ (factory function) + copyRange for proper object creation
// No runtime functions are needed - the methods are marked as IsLowered
// in primordials.go so codegen doesn't generate wrapper functions for them

/// IndexOf: zeus_array_indexOf(this_ptr, return_buffer_ptr_ptr, value_ptr)
/// Returns the index of the first occurrence of value, or -1 if not found
export fn zeus_array_indexOf(this_ptr: *anyopaque, return_buffer_ptr_ptr: ?*anyopaque, value_ptr: *anyopaque) callconv(.C) void {
    const this_array = castToArrayObj(this_ptr);
    const element_size = getElementSize(this_array);
    const type_info = this_array.obj_header.getObjectTypeInfo();
    const element_type = type_info.array_element_type;

    var result: i32 = -1;

    if (this_array.length > 0) {
        if (getDataBytes(this_array)) |data_bytes| {
            const value_bytes = @as([*]const u8, @ptrCast(@alignCast(value_ptr)));
            var i: u32 = 0;
            while (i < this_array.length) : (i += 1) {
                const offset = getElementOffset(i, element_size);
                if (elementsEqual(data_bytes + offset, value_bytes, element_size, element_type)) {
                    result = @intCast(i);
                    break;
                }
            }
        }
    }

    // Return the result
    if (runtime_util.allocateReturnBuffer(return_buffer_ptr_ptr, @sizeOf(i32))) |result_bytes| {
        const result_ptr = @as(*i32, @ptrCast(@alignCast(result_bytes.ptr)));
        result_ptr.* = result;
    }
}

/// LastIndexOf: zeus_array_lastIndexOf(this_ptr, return_buffer_ptr_ptr, value_ptr)
/// Returns the index of the last occurrence of value, or -1 if not found
export fn zeus_array_lastIndexOf(this_ptr: *anyopaque, return_buffer_ptr_ptr: ?*anyopaque, value_ptr: *anyopaque) callconv(.C) void {
    const this_array = castToArrayObj(this_ptr);
    const element_size = getElementSize(this_array);
    const type_info = this_array.obj_header.getObjectTypeInfo();
    const element_type = type_info.array_element_type;

    var result: i32 = -1;

    if (this_array.length > 0) {
        if (getDataBytes(this_array)) |data_bytes| {
            const value_bytes = @as([*]const u8, @ptrCast(@alignCast(value_ptr)));
            // Search backwards
            var i: u32 = this_array.length;
            while (i > 0) {
                i -= 1;
                const offset = getElementOffset(i, element_size);
                if (elementsEqual(data_bytes + offset, value_bytes, element_size, element_type)) {
                    result = @intCast(i);
                    break;
                }
            }
        }
    }

    // Return the result
    if (runtime_util.allocateReturnBuffer(return_buffer_ptr_ptr, @sizeOf(i32))) |result_bytes| {
        const result_ptr = @as(*i32, @ptrCast(@alignCast(result_bytes.ptr)));
        result_ptr.* = result;
    }
}

/// Find: zeus_array_find(this_ptr, return_buffer_ptr_ptr, value_ptr)
/// Returns the first element that equals value, or default value if not found
export fn zeus_array_find(this_ptr: *anyopaque, return_buffer_ptr_ptr: ?*anyopaque, value_ptr: *anyopaque) callconv(.C) void {
    const this_array = castToArrayObj(this_ptr);
    const element_size = getElementSize(this_array);
    const type_info = this_array.obj_header.getObjectTypeInfo();
    const element_type = type_info.array_element_type;

    // Allocate return buffer for the element
    const result_bytes = runtime_util.allocateReturnBuffer(return_buffer_ptr_ptr, element_size);
    if (result_bytes == null) {
        return;
    }

    // Initialize to default value (zero)
    @memset(result_bytes.?[0..element_size], 0);

    if (this_array.length > 0) {
        if (getDataBytes(this_array)) |data_bytes| {
            const value_bytes = @as([*]const u8, @ptrCast(@alignCast(value_ptr)));
            // Search forwards
            for (0..this_array.length) |i| {
                const offset = getElementOffset(@intCast(i), element_size);
                if (elementsEqual(data_bytes + offset, value_bytes, element_size, element_type)) {
                    // Found it - copy the element to result
                    @memcpy(result_bytes.?[0..element_size], data_bytes[offset .. offset + element_size]);
                    return;
                }
            }
        }
    }
    // Not found - result is already set to default (zero)
}

/// FindIndex: zeus_array_findIndex(this_ptr, return_buffer_ptr_ptr, value_ptr)
/// Returns the index of first element that equals value, or -1 if not found
/// (Same behavior as indexOf, provided for API consistency)
export fn zeus_array_findIndex(this_ptr: *anyopaque, return_buffer_ptr_ptr: ?*anyopaque, value_ptr: *anyopaque) callconv(.C) void {
    const this_array = castToArrayObj(this_ptr);
    const element_size = getElementSize(this_array);
    const type_info = this_array.obj_header.getObjectTypeInfo();
    const element_type = type_info.array_element_type;

    var result: i32 = -1;

    if (this_array.length > 0) {
        if (getDataBytes(this_array)) |data_bytes| {
            const value_bytes = @as([*]const u8, @ptrCast(@alignCast(value_ptr)));
            // Search forwards
            for (0..this_array.length) |i| {
                const offset = getElementOffset(@intCast(i), element_size);
                if (elementsEqual(data_bytes + offset, value_bytes, element_size, element_type)) {
                    result = @intCast(i);
                    break;
                }
            }
        }
    }

    // Return the result
    if (runtime_util.allocateReturnBuffer(return_buffer_ptr_ptr, @sizeOf(i32))) |result_bytes| {
        const result_ptr = @as(*i32, @ptrCast(@alignCast(result_bytes.ptr)));
        result_ptr.* = result;
    }
}

/// Includes: zeus_array_includes(this_ptr, return_buffer_ptr_ptr, value_ptr)
/// Returns true if array contains the value, false otherwise
export fn zeus_array_includes(this_ptr: *anyopaque, return_buffer_ptr_ptr: ?*anyopaque, value_ptr: *anyopaque) callconv(.C) void {
    const this_array = castToArrayObj(this_ptr);
    const element_size = getElementSize(this_array);
    const type_info = this_array.obj_header.getObjectTypeInfo();
    const element_type = type_info.array_element_type;

    var result: bool = false;

    if (this_array.length > 0) {
        if (getDataBytes(this_array)) |data_bytes| {
            const value_bytes = @as([*]const u8, @ptrCast(@alignCast(value_ptr)));
            var i: u32 = 0;
            while (i < this_array.length) : (i += 1) {
                const offset = getElementOffset(i, element_size);
                if (elementsEqual(data_bytes + offset, value_bytes, element_size, element_type)) {
                    result = true;
                    break;
                }
            }
        }
    }

    // Return the result
    if (runtime_util.allocateReturnBuffer(return_buffer_ptr_ptr, @sizeOf(bool))) |result_bytes| {
        result_bytes[0] = if (result) 1 else 0;
    }
}

/// CopyRangeReversed: zeus_array_copyRangeReversed(this_ptr, return_buffer_ptr, source_ptr_ptr, src_offset_ptr, dest_offset_ptr, count_ptr)
/// Copies a range of elements from source array to this array in REVERSE order
/// This is used by the lowered reverse() operation
/// Elements at source[srcOffset..srcOffset+count] are copied to dest[destOffset..destOffset+count] in reverse
export fn zeus_array_copyRangeReversed(this_ptr: *anyopaque, return_buffer_ptr: ?*anyopaque, source_ptr_ptr: *anyopaque, src_offset_ptr: *anyopaque, dest_offset_ptr: *anyopaque, count_ptr: *anyopaque) callconv(.C) void {
    _ = return_buffer_ptr; // void return, not used

    const dest_array = castToArrayObj(this_ptr);

    // Dereference to get the source array pointer (passed via alloca)
    const source_obj_ptr = @as(**anyopaque, @ptrCast(@alignCast(source_ptr_ptr))).*;
    const source_array = castToArrayObj(source_obj_ptr);

    // Get offset and count values
    const src_offset_val = @as(*i32, @ptrCast(@alignCast(src_offset_ptr))).*;
    const dest_offset_val = @as(*i32, @ptrCast(@alignCast(dest_offset_ptr))).*;
    const count_val = @as(*i32, @ptrCast(@alignCast(count_ptr))).*;

    // Validate and convert to unsigned
    if (src_offset_val < 0 or dest_offset_val < 0 or count_val <= 0) {
        return;
    }

    const src_offset: u32 = @intCast(src_offset_val);
    const dest_offset: u32 = @intCast(dest_offset_val);
    const count: u32 = @intCast(count_val);

    const element_size = getElementSize(dest_array);

    // Bounds check on source
    if (src_offset + count > source_array.length) {
        debug.log(allocator, "array_copyRangeReversed", "source range {}..{} exceeds length {}", .{ src_offset, src_offset + count, source_array.length });
        return;
    }

    // Ensure destination has enough capacity
    const required_length = dest_offset + count;
    if (dest_array.capacity < required_length) {
        if (!resizeArray(dest_array, required_length)) {
            return;
        }
    }

    // Copy the data in REVERSE order
    if (source_array.data != null and dest_array.data != null and count > 0) {
        const src_bytes = getDataBytes(source_array).?;
        const dest_bytes = getDataBytes(dest_array).?;

        var i: u32 = 0;
        while (i < count) : (i += 1) {
            // Source index: reading from end towards start
            const src_idx = src_offset + count - 1 - i;
            // Dest index: writing from start towards end
            const dest_idx = dest_offset + i;

            const src_byte_offset = getElementOffset(src_idx, element_size);
            const dest_byte_offset = getElementOffset(dest_idx, element_size);
            @memcpy(dest_bytes[dest_byte_offset .. dest_byte_offset + element_size], src_bytes[src_byte_offset .. src_byte_offset + element_size]);
        }
    }

    // Update destination length if we extended it
    if (required_length > dest_array.length) {
        dest_array.length = required_length;
    }

    debug.log(allocator, "array_copyRangeReversed", "copied {} elements in reverse from src[{}..] to dest[{}..], new length={}", .{ count, src_offset, dest_offset, dest_array.length });
}

/// Fill: zeus_array_fill(this_ptr, return_buffer_ptr, value_ptr)
/// Fills all elements with the given value
export fn zeus_array_fill(this_ptr: *anyopaque, return_buffer_ptr: ?*anyopaque, value_ptr: *anyopaque) callconv(.C) void {
    _ = return_buffer_ptr; // void return

    const this_array = castToArrayObj(this_ptr);
    if (this_array.length == 0) return;

    const element_size = getElementSize(this_array);
    if (getDataBytes(this_array)) |data_bytes| {
        const value_bytes = @as([*]const u8, @ptrCast(@alignCast(value_ptr)));
        var i: u32 = 0;
        while (i < this_array.length) : (i += 1) {
            const offset = getElementOffset(i, element_size);
            @memcpy(data_bytes[offset .. offset + element_size], value_bytes[0..element_size]);
        }
    }

    debug.log(allocator, "array_fill", "filled {} elements", .{this_array.length});
}

/// Clear: zeus_array_clear(this_ptr, return_buffer_ptr)
/// Clears all elements (sets length to 0)
export fn zeus_array_clear(this_ptr: *anyopaque, return_buffer_ptr: ?*anyopaque) callconv(.C) void {
    _ = return_buffer_ptr; // void return

    const this_array = castToArrayObj(this_ptr);
    this_array.length = 0;

    debug.log(allocator, "array_clear", "cleared array", .{});
}

/// IsEmpty: zeus_array_isEmpty(this_ptr, return_buffer_ptr_ptr)
/// Returns true if length is 0
export fn zeus_array_isEmpty(this_ptr: *anyopaque, return_buffer_ptr_ptr: ?*anyopaque) callconv(.C) void {
    const this_array = castToArrayObj(this_ptr);
    const result: bool = this_array.length == 0;

    // Return the result
    if (runtime_util.allocateReturnBuffer(return_buffer_ptr_ptr, @sizeOf(bool))) |result_bytes| {
        result_bytes[0] = if (result) 1 else 0;
    }
}
