// Runtime utility functions for Zeus runtime
// Provides common patterns used across runtime functions

const std = @import("std");
const abi = @import("abi.zig");

// Forward declaration - will be resolved at link time
extern fn zeus_gc_alloc(size: u32) ?*anyopaque;

// Default object type info for generic Zeus objects
// This is used for wrapper objects that don't belong to a specific class
var default_object_type_info: abi.ZeusObjectTypeInfo = abi.ZeusObjectTypeInfo{
    .object_type_id = 0,
    .object_type = abi.ZeusObjectType.object,
    .array_element_type = abi.ZeusType._null,
    .parent_type_info = null,
};

// Dummy vtable for default object header
var default_vtable: u8 = 0;

// Default object header for generic Zeus objects
var default_object_header: abi.ZeusObjectHeader = abi.ZeusObjectHeader{
    .vtable = @ptrCast(&default_vtable),
    .object_type_info = @ptrCast(&default_object_type_info),
    .gc_offsets_count = 0,
};

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

    const obj_bytes = @as([*]u8, @ptrCast(@alignCast(wrapper_obj.?)));

    // Set the header pointer to point to our default object header
    const zeus_obj = @as(*abi.ZeusObj, @ptrCast(@alignCast(wrapper_obj.?)));
    zeus_obj.obj_header = &default_object_header;

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

    const obj_bytes = @as([*]u8, @ptrCast(@alignCast(wrapper_obj.?)));

    // Set the header pointer to point to our default object header
    const zeus_obj = @as(*abi.ZeusObj, @ptrCast(@alignCast(wrapper_obj.?)));
    zeus_obj.obj_header = &default_object_header;

    // Zero out the result field (after the header pointer)
    @memset(obj_bytes[header_ptr_size..total_size], 0);

    // Store the pointer to the wrapper object in the ptr-ptr location
    const ptr_ptr = @as(*?*anyopaque, @ptrCast(@alignCast(return_buffer_ptr_ptr.?)));
    ptr_ptr.* = wrapper_obj;

    return true;
}

/// Allocates raw bytes using the provided allocator (not GC-tracked)
/// Use this for allocating data buffers that should not be tracked by the garbage collector
///
/// Parameters:
/// - allocator: The allocator to use for the allocation
/// - size: Size in bytes to allocate
///
/// Returns:
/// - A pointer to the allocated memory, or null if allocation failed
pub fn allocateRawBytes(allocator: std.mem.Allocator, size: u32) ?*anyopaque {
    if (size == 0) {
        return null;
    }

    const bytes = allocator.alloc(u8, size) catch {
        return null;
    };

    return @ptrCast(bytes.ptr);
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
