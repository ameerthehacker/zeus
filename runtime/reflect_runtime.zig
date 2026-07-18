// Zeus runtime reflection printer.
//
// zeus_reflect_to_string(obj) renders any reference value (object / array / string) to a debug
// string like `ClassName { field: value, ... }` or `[e0, e1, ...]`, driven entirely by the per-class
// type-info metadata codegen emits (class name + a field table of {name, offset, type_tag, signed}).
// Object/array fields recurse through each value's OWN header, so polymorphic and interface values
// print as their concrete runtime type. Primitives are formatted inline; nested strings are quoted;
// recursion is bounded by a depth limit (a cycle guard for self-referential graphs).
//
// This is the single runtime entry point behind the "log anything" experience: the type checker's
// emitToString lowers `console.log(x)` / `"s" + x` / `` `${x}` `` to a call here whenever x has no
// user-defined toString. All formatting lives here; codegen only emits the metadata + the call.

const std = @import("std");
const abi = @import("abi.zig");
const string_runtime = @import("string_runtime.zig");
const number_runtime = @import("number_runtime.zig");
const runtime_util = @import("runtime_util.zig");

// Fast small allocations for the transient output buffer (matches array_runtime).
const allocator = std.heap.c_allocator;

// Bounds recursion for self-referential graphs (linked lists, trees, parent/child). Beyond this depth
// we print an ellipsis instead of recursing further.
const MAX_DEPTH: i32 = 8;

const Buf = std.ArrayList(u8);

// Explicit error set: `reflect` recurses, so Zig cannot infer it.
const Error = std.mem.Allocator.Error;

inline fn readAt(comptime T: type, ptr: [*]const u8) T {
    return @as(*const T, @ptrCast(@alignCast(ptr))).*;
}

/// True if a ZeusType tag denotes a heap reference (a pointer to recurse into) rather than an inline
/// primitive read by value.
inline fn isPointerTag(tag: abi.ZeusType) bool {
    return tag == .object;
}

/// Append a primitive field/element, read from `ptr` (a naturally-aligned location) and formatted by
/// its ZeusType tag. `is_signed` disambiguates signed vs unsigned integers (ZeusType has no split).
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
        // Object pointers recurse elsewhere; _null shouldn't appear as a concrete field/element.
        .object, ._null => "?",
    };
    try out.appendSlice(text);
}

/// Render an object as `ClassName { field: value, ... }` using its field table (base-first, incl.
/// inherited). Object/array/string fields recurse via the field value's own header.
fn reflectObject(out: *Buf, obj_ptr: *anyopaque, type_info: *abi.ZeusObjectTypeInfo, depth: i32) Error!void {
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
            try reflect(out, readAt(?*anyopaque, field_ptr), depth - 1);
        } else {
            try appendPrimitive(out, tag, field.is_signed != 0, field_ptr);
        }
    }
    try out.appendSlice(" }");
}

/// Render an array as `[e0, e1, ...]`, striding the backing buffer by the element size. Primitive
/// elements are formatted inline; object/array/string elements recurse via their own header.
fn reflectArray(out: *Buf, obj_ptr: *anyopaque, type_info: *abi.ZeusObjectTypeInfo, depth: i32) Error!void {
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
                try reflect(out, readAt(?*anyopaque, elem_ptr), depth - 1);
            } else {
                // array_element_type carries no signedness; format primitive elements as signed
                // (correct for i-typed and float arrays; unsigned-element arrays are a v1 nuance).
                try appendPrimitive(out, element_type, true, elem_ptr);
            }
        }
    }
    try out.append(']');
}

/// Dispatch on an object's runtime kind (from its header) and render it. Null → "null"; strings are
/// quoted (only reached when nested — a top-level string never goes through reflection).
fn reflect(out: *Buf, obj_ptr: ?*anyopaque, depth: i32) Error!void {
    const ptr = obj_ptr orelse {
        try out.appendSlice("null");
        return;
    };
    if (depth <= 0) {
        try out.appendSlice("...");
        return;
    }
    const obj = @as(*abi.ZeusObj, @ptrCast(@alignCast(ptr)));
    const type_info = obj.obj_header.getObjectTypeInfo();
    switch (type_info.object_type) {
        .string => {
            try out.append('"');
            const str = string_runtime.castToStringObj(ptr);
            if (str.data) |str_array| {
                if (str_array.data) |str_bytes| {
                    const slice = @as([*]const u8, @ptrCast(@alignCast(str_bytes)))[0..@intCast(str_array.length)];
                    try out.appendSlice(slice);
                }
            }
            try out.append('"');
        },
        .array => try reflectArray(out, ptr, type_info, depth),
        .object => try reflectObject(out, ptr, type_info, depth),
    }
}

/// zeus_reflect_to_string(obj): string — the runtime reflection printer. ABI: [this_ptr(unused),
/// return_buffer_ptr_ptr, obj_ptr_ptr] where obj_ptr_ptr points at the object pointer.
export fn zeus_reflect_to_string(this_ptr: ?*anyopaque, return_buffer_ptr_ptr: ?*anyopaque, obj_ptr_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    const obj_ptr = @as(*?*anyopaque, @ptrCast(@alignCast(obj_ptr_ptr))).*;

    var out = Buf.init(allocator);
    defer out.deinit();
    reflect(&out, obj_ptr, MAX_DEPTH) catch {};

    number_runtime.writeStringResult(return_buffer_ptr_ptr, number_runtime.makeString(out.items));
}
