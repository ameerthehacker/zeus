const std = @import("std");

var gpa = std.heap.GeneralPurposeAllocator(.{}){};
const allocator = gpa.allocator();

export fn gc_alloc(size: u32) ?*anyopaque {
    const bytes = allocator.alloc(u8, size) catch return null;
    return bytes.ptr;
}
