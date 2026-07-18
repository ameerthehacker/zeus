// The default `toString` body shared by every class that doesn't define its own. zeus_default_toString
// renders `this` structurally (`ClassName { field: value }` / `[e0, ...]`) from the emitted type-info
// metadata; nested object/array fields dispatch to their OWN toString via the vtable, so user toStrings
// compose. A threadlocal depth bounds self-referential graphs.

const std = @import("std");
const abi = @import("abi.zig");
const string_runtime = @import("string_runtime.zig");
const number_runtime = @import("number_runtime.zig");
const runtime_util = @import("runtime_util.zig");

const allocator = std.heap.c_allocator;
const MAX_DEPTH: i32 = 16;
threadlocal var reflect_depth: i32 = 0;

const Buf = std.ArrayList(u8);
const Error = std.mem.Allocator.Error;

inline fn readAt(comptime T: type, ptr: [*]const u8) T {
    return @as(*const T, @ptrCast(@alignCast(ptr))).*;
}

inline fn isPointerTag(tag: abi.ZeusType) bool {
    return tag == .object;
}

fn appendPrimitive(out: *Buf, tag: abi.ZeusType, is_signed: bool, ptr: [*]const u8) Error!void {
    var buf: [512]u8 = undefined;
    const text: []const u8 = switch (tag) {
        ._i8 => if (is_signed) number_runtime.formatScalar(i8, &buf, readAt(i8, ptr)) else number_runtime.formatScalar(u8, &buf, readAt(u8, ptr)),
        ._i16 => if (is_signed) number_runtime.formatScalar(i16, &buf, readAt(i16, ptr)) else number_runtime.formatScalar(u16, &buf, readAt(u16, ptr)),
        ._i32 => if (is_signed) number_runtime.formatScalar(i32, &buf, readAt(i32, ptr)) else number_runtime.formatScalar(u32, &buf, readAt(u32, ptr)),
        ._i64 => if (is_signed) number_runtime.formatScalar(i64, &buf, readAt(i64, ptr)) else number_runtime.formatScalar(u64, &buf, readAt(u64, ptr)),
        ._f32 => number_runtime.formatScalar(f32, &buf, readAt(f32, ptr)),
        ._f64 => number_runtime.formatScalar(f64, &buf, readAt(f64, ptr)),
        ._bool => if (readAt(bool, ptr)) "true" else "false",
        .object, ._null => "?",
    };
    try out.appendSlice(text);
}

fn appendStringBytes(out: *Buf, str_ptr: *anyopaque) Error!void {
    const str = string_runtime.castToStringObj(str_ptr);
    if (str.data) |str_array| {
        if (str_array.data) |str_bytes| {
            try out.appendSlice(@as([*]const u8, @ptrCast(@alignCast(str_bytes)))[0..@intCast(str_array.length)]);
        }
    }
}

// Call a value's own toString (vtable slot) and append its result raw.
fn appendViaToString(out: *Buf, obj_ptr: *anyopaque, slot: i32) Error!void {
    const obj = @as(*abi.ZeusObj, @ptrCast(@alignCast(obj_ptr)));
    const vtable_slots = @as([*]const *anyopaque, @ptrCast(@alignCast(obj.obj_header.vtable)));
    const tostring = @as(*const fn (*anyopaque) callconv(.C) ?*anyopaque, @ptrCast(@alignCast(vtable_slots[@intCast(slot)])));
    if (tostring(obj_ptr)) |str_ptr| try appendStringBytes(out, str_ptr);
}

// A nested field/element: null, a quoted string, its own toString (if any), else structural.
fn renderNested(out: *Buf, value: ?*anyopaque) Error!void {
    const ptr = value orelse {
        try out.appendSlice("null");
        return;
    };
    const type_info = @as(*abi.ZeusObj, @ptrCast(@alignCast(ptr))).obj_header.getObjectTypeInfo();
    if (type_info.object_type == .string) {
        try out.append('"');
        try appendStringBytes(out, ptr);
        try out.append('"');
        return;
    }
    if (type_info.tostring_slot >= 0) {
        try appendViaToString(out, ptr, type_info.tostring_slot);
    } else {
        try renderStructural(out, ptr);
    }
}

fn reflectObject(out: *Buf, obj_ptr: *anyopaque, type_info: *abi.ZeusObjectTypeInfo) Error!void {
    try out.appendSlice(std.mem.span(type_info.class_name));
    if (type_info.num_fields == 0 or type_info.field_table == null) {
        try out.appendSlice(" {}");
        return;
    }
    try out.appendSlice(" { ");
    const base = @as([*]const u8, @ptrCast(@alignCast(obj_ptr)));
    const fields = type_info.field_table.?;
    var i: u32 = 0;
    while (i < type_info.num_fields) : (i += 1) {
        if (i > 0) try out.appendSlice(", ");
        const field = fields[i];
        try out.appendSlice(std.mem.span(field.name));
        try out.appendSlice(": ");
        const field_ptr = base + field.offset;
        const tag = @as(abi.ZeusType, @enumFromInt(field.type_tag));
        if (isPointerTag(tag)) {
            try renderNested(out, readAt(?*anyopaque, field_ptr));
        } else {
            try appendPrimitive(out, tag, field.is_signed != 0, field_ptr);
        }
    }
    try out.appendSlice(" }");
}

fn reflectArray(out: *Buf, obj_ptr: *anyopaque, type_info: *abi.ZeusObjectTypeInfo) Error!void {
    const array = runtime_util.castToArrayObj(obj_ptr);
    const element_type = type_info.array_element_type;
    const element_size = runtime_util.getZeusTypeSize(element_type);
    try out.append('[');
    if (array.data) |data| {
        const bytes = @as([*]const u8, @ptrCast(@alignCast(data)));
        var i: u32 = 0;
        while (i < array.length) : (i += 1) {
            if (i > 0) try out.appendSlice(", ");
            const elem_ptr = bytes + @as(usize, @intCast(i)) * element_size;
            if (isPointerTag(element_type)) {
                try renderNested(out, readAt(?*anyopaque, elem_ptr));
            } else {
                // array_element_type carries no signedness; unsigned-element arrays are a v1 nuance.
                try appendPrimitive(out, element_type, true, elem_ptr);
            }
        }
    }
    try out.append(']');
}

// Structural render of a value (ClassName {...} / [..] / string bytes). The threadlocal depth guard
// lives here so it bounds ALL reflection recursion, including nested no-toString objects and cycles.
fn renderStructural(out: *Buf, this_ptr: *anyopaque) Error!void {
    reflect_depth += 1;
    defer reflect_depth -= 1;
    if (reflect_depth > MAX_DEPTH) {
        try out.appendSlice("...");
        return;
    }
    const type_info = @as(*abi.ZeusObj, @ptrCast(@alignCast(this_ptr))).obj_header.getObjectTypeInfo();
    switch (type_info.object_type) {
        .string => try appendStringBytes(out, this_ptr),
        .array => try reflectArray(out, this_ptr, type_info),
        .object => try reflectObject(out, this_ptr, type_info),
    }
}

fn renderToString(this_ptr: *anyopaque, return_buffer_ptr_ptr: ?*anyopaque) void {
    var out = Buf.init(allocator);
    defer out.deinit();
    renderStructural(&out, this_ptr) catch {};
    number_runtime.writeStringResult(return_buffer_ptr_ptr, number_runtime.makeString(out.items));
}

/// zeus_reflect_to_string(obj): string — the reflection printer behind the implicit `T -> string`
/// conversion (emitToString) and universal `x.toString()`. Renders obj structurally; nested object/array
/// fields dispatch to their own toString via the vtable, so custom toStrings compose. null → "null".
export fn zeus_reflect_to_string(this_ptr: ?*anyopaque, return_buffer_ptr_ptr: ?*anyopaque, obj_ptr_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    if (@as(*?*anyopaque, @ptrCast(@alignCast(obj_ptr_ptr))).*) |p| {
        renderToString(p, return_buffer_ptr_ptr);
    } else {
        number_runtime.writeStringResult(return_buffer_ptr_ptr, number_runtime.makeString("null"));
    }
}
