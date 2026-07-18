// This file defines the Application Binary Interface (ABI) for Zeus objects.
// It describes how Zeus objects are represented in memory and provides
// the interface we use to interact with objects created in the Zeus language.
const std = @import("std");

pub const ZeusObjectType = enum(u8) {
    object = 0,
    array = 1,
    string = 2,
};

pub const ZeusType = enum(u8) { _i8 = 0, _i16 = 1, _i32 = 2, _i64 = 3, _f32 = 4, _f64 = 5, _bool = 6, object = 7, _null = 8 };

// One entry in a class's reflection field table. Must match codegen's ZeusFieldInfo (field order +
// widths): {name: i8*, offset: u32, type_tag: u8, is_signed: u8}.
pub const ZeusFieldInfo = extern struct {
    // NUL-terminated field name
    name: [*:0]const u8,
    // byte offset of the field within the object struct
    offset: u32,
    // ZeusType tag describing how to read/format the field
    type_tag: u8,
    // 1 if the field is a signed integer, else 0 (ZeusType has no signed/unsigned distinction)
    is_signed: u8,
};

pub const ZeusObjectTypeInfo = extern struct {
    // unique id given to every class in the program
    // primordial classes with have a known fixed id
    // i32-wide to match codegen's ZeusObjectTypeInfo (interface dispatch keys on this id, so
    // the class-id space must not be capped at 255).
    object_type_id: u32,
    object_type: ZeusObjectType,
    // if the object is an array, this is the type of the elements in the array
    // if it is not an array then this will have _null type
    array_element_type: ZeusType,
    parent_type_info: ?*anyopaque,
    // Reflection metadata (consumed by zeus_reflect_to_string). class_name is always set; field_table
    // is null for arrays and field-less classes (arrays render via length/array_element_type).
    class_name: [*:0]const u8,
    field_table: ?[*]const ZeusFieldInfo,
    num_fields: u32,

    pub fn getParentTypeInfo(self: *const ZeusObjectTypeInfo) *ZeusObjectTypeInfo {
        return @as(*ZeusObjectTypeInfo, @ptrCast(@alignCast(self.parent_type_info)));
    }
};

// Header of the zeus object
pub const ZeusObjectHeader = extern struct {
    vtable: *anyopaque,
    object_type_info: *anyopaque,

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
