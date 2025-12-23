// Runtime utility functions for Zeus runtime
// Provides common patterns used across runtime functions

const std = @import("std");
const abi = @import("abi.zig");

// Forward declaration - will be resolved at link time
extern fn zeus_gc_alloc(size: u32) ?*anyopaque;

/// Allocates memory for a return value following Zeus object ABI
/// Creates a wrapper object with a header and one field containing the result
/// Returns a byte slice pointing to the result field for the caller to write
///
/// Parameters:
/// - return_buffer_ptr_ptr: Pointer to a pointer where the allocated object address will be stored
/// - size: Size in bytes to allocate for the result field
///
/// Returns:
/// - A byte slice to the result field, or null if allocation failed or ptr-ptr is null
pub fn allocateReturnBuffer(return_buffer_ptr_ptr: ?*anyopaque, size: u32) ?[]u8 {
    if (return_buffer_ptr_ptr == null or size == 0) {
        return null;
    }

    // Calculate total size: header pointer + result field
    const header_ptr_size = @sizeOf(*abi.ZeusObjectHeader);
    const total_size = header_ptr_size + size;

    // Allocate memory for the wrapper object using GC allocator
    const wrapper_obj = zeus_gc_alloc(total_size);
    if (wrapper_obj == null) {
        return null;
    }

    // Zero out the header pointer (first field)
    const obj_bytes = @as([*]u8, @ptrCast(@alignCast(wrapper_obj.?)));
    @memset(obj_bytes[0..header_ptr_size], 0);

    // Store the pointer to the wrapper object in the ptr-ptr location
    const ptr_ptr = @as(*?*anyopaque, @ptrCast(@alignCast(return_buffer_ptr_ptr.?)));
    ptr_ptr.* = wrapper_obj;

    // Return a byte slice to the result field (after the header pointer)
    return obj_bytes[header_ptr_size..total_size];
}

/// Allocates memory and initializes it with zeros, following Zeus object ABI
/// Creates a wrapper object with a header and one field containing the result
///
/// Parameters:
/// - return_buffer_ptr_ptr: Pointer to a pointer where the allocated object address will be stored
/// - size: Size in bytes to allocate for the result field
///
/// Returns:
/// - true if successful, false otherwise
pub fn allocateZeroedReturnBuffer(return_buffer_ptr_ptr: ?*anyopaque, size: u32) bool {
    if (return_buffer_ptr_ptr == null or size == 0) {
        return false;
    }

    // Calculate total size: header pointer + result field
    const header_ptr_size = @sizeOf(*abi.ZeusObjectHeader);
    const total_size = header_ptr_size + size;

    // Allocate memory for the wrapper object using GC allocator
    const wrapper_obj = zeus_gc_alloc(total_size);
    if (wrapper_obj == null) {
        return false;
    }

    // Zero out the entire object
    const obj_bytes = @as([*]u8, @ptrCast(@alignCast(wrapper_obj.?)));
    @memset(obj_bytes[0..total_size], 0);

    // Store the pointer to the wrapper object in the ptr-ptr location
    const ptr_ptr = @as(*?*anyopaque, @ptrCast(@alignCast(return_buffer_ptr_ptr.?)));
    ptr_ptr.* = wrapper_obj;

    return true;
}

pub fn getZeusTypeSize(zeus_type: abi.ZeusType) u32 {
    switch (zeus_type) {
        ._i8 => return @sizeOf(i8),
        ._i16 => return @sizeOf(i16),
        ._i32 => return @sizeOf(i32),
        ._i64 => return @sizeOf(i64),
        ._f32 => return @sizeOf(f32),
        ._f64 => return @sizeOf(f64),
        ._bool => return @sizeOf(bool),
        .object => return @sizeOf(*anyopaque),
        ._null => return 0,
    }
}
