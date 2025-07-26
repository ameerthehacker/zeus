// This file defines the Application Binary Interface (ABI) for Zeus objects.
// It describes how Zeus objects are represented in memory and provides
// the interface we use to interact with objects created in the Zeus language.
const std = @import("std");

// Zeus class ABI struct - matches LLVM layout: { ptr, i8, [N x i8] }
pub const ZeusObjectHeader = struct {
    vtable: *anyopaque,
    gc_offsets_count: u8,
    // Note: gc_offsets are stored as an inline array immediately after gc_offsets_count within this struct

    pub fn getGcOffsets(self: *const ZeusObjectHeader) []const u8 {
        // The gc_offsets array is embedded right after gc_offsets_count in the LLVM struct
        const self_ptr = @as([*]const u8, @ptrCast(self));
        const offsets_start = self_ptr + @sizeOf(*anyopaque) + @sizeOf(u8);
        return offsets_start[0..self.gc_offsets_count];
    }
};

// Structure to track GC roots - just the pointer and whether it's marked
pub const ZeusObj = struct {
    obj_header: *ZeusObjectHeader,
};
