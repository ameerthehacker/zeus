// Runtime utility functions for Zeus runtime
// Provides common patterns used across runtime functions

const std = @import("std");

// Forward declaration - will be resolved at link time
extern fn zeus_gc_alloc(size: u32) ?*anyopaque;

/// Allocates memory for a return value and stores the pointer in the provided ptr-ptr location
/// Returns a byte slice pointing to the allocated memory for the caller to write the result
/// 
/// Parameters:
/// - return_buffer_ptr_ptr: Pointer to a pointer where the allocated memory address will be stored
/// - size: Size in bytes to allocate
/// 
/// Returns:
/// - A byte slice to the allocated memory, or null if allocation failed or ptr-ptr is null
pub fn allocateReturnBuffer(return_buffer_ptr_ptr: ?*anyopaque, size: u32) ?[]u8 {
    if (return_buffer_ptr_ptr == null or size == 0) {
        return null;
    }

    // Allocate memory for the result using GC allocator
    const result_buffer = zeus_gc_alloc(size);
    if (result_buffer == null) {
        return null;
    }

    // Store the pointer to the allocated buffer in the ptr-ptr location
    const ptr_ptr = @as(*?*anyopaque, @ptrCast(@alignCast(return_buffer_ptr_ptr.?)));
    ptr_ptr.* = result_buffer;

    // Return a byte slice for the caller to write to
    const result_bytes = @as([*]u8, @ptrCast(@alignCast(result_buffer.?)));
    return result_bytes[0..size];
}

/// Allocates memory and initializes it with zeros, then stores the pointer
/// Convenience function for returning default/zero values
/// 
/// Parameters:
/// - return_buffer_ptr_ptr: Pointer to a pointer where the allocated memory address will be stored
/// - size: Size in bytes to allocate
/// 
/// Returns:
/// - true if successful, false otherwise
pub fn allocateZeroedReturnBuffer(return_buffer_ptr_ptr: ?*anyopaque, size: u32) bool {
    const buffer = allocateReturnBuffer(return_buffer_ptr_ptr, size) orelse return false;
    @memset(buffer, 0);
    return true;
}

