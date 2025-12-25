// This file defines the Application Binary Interface (ABI) for Zeus objects.
// It describes how Zeus objects are represented in memory and provides
// the interface we use to interact with objects created in the Zeus language.
const std = @import("std");

pub const ZeusObjectType = enum(u8) {
    object = 0,
    array = 1,
};

pub const ZeusType = enum(u8) { _i8 = 0, _i16 = 1, _i32 = 2, _i64 = 3, _f32 = 4, _f64 = 5, _bool = 6, object = 7, _null = 8 };

pub const ZeusObjectTypeInfo = extern struct {
    // unique id given to every class in the program
    // primordial classes with have a known fixed id
    object_type_id: u8,
    object_type: ZeusObjectType,
    // if the object is an array, this is the type of the elements in the array
    // if it is not an array then this will have _null type
    array_element_type: ZeusType,
    parent_type_info: ?*anyopaque,

    pub fn getParentTypeInfo(self: *const ZeusObjectTypeInfo) *ZeusObjectTypeInfo {
        return @as(*ZeusObjectTypeInfo, @ptrCast(@alignCast(self.parent_type_info)));
    }
};

// Header of the zeus object
pub const ZeusObjectHeader = extern struct {
    vtable: *anyopaque,
    object_type_info: *anyopaque,
    gc_offsets_count: u8,
    // Note: gc_offsets are stored as an inline array immediately after gc_offsets_count within this struct

    pub fn getGcOffsets(self: *const ZeusObjectHeader) []const u8 {
        // The gc_offsets array is embedded right after gc_offsets_count in the LLVM struct
        const self_ptr = @as([*]const u8, @ptrCast(self));
        // 2 * @sizeOf(*anyopaque) because the first two fields are the vtable and object type info
        const offsets_start = self_ptr + 2 * @sizeOf(*anyopaque) + @sizeOf(u8);
        return offsets_start[0..self.gc_offsets_count];
    }

    pub fn getObjectTypeInfo(self: *const ZeusObjectHeader) *ZeusObjectTypeInfo {
        return @as(*ZeusObjectTypeInfo, @ptrCast(@alignCast(self.object_type_info)));
    }
};

// Zeus object ABI
pub const ZeusObj = extern struct {
    obj_header: *ZeusObjectHeader,
};

pub const ZeusArrayObj = extern struct {
    obj_header: *ZeusObjectHeader,
    capacity: u32,
    length: u32,
    data: ?*anyopaque,
};
