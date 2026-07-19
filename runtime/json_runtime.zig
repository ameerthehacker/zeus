// Zeus JSON runtime — backs the global JSON / JsonValue primordials (see internal/prelude/json.zs).
// The document is a tree of GC-allocated JsonNodes; each JsonValue wraps a node pointer in its
// (GC-scanned) `node` field, so Boehm traces the whole tree while any JsonValue into it is live.
// Method fat-ABI: (this_ptr, return_buffer, ...arg_ptr_ptrs).

const std = @import("std");
const runtime_util = @import("runtime_util.zig");
const string_runtime = @import("string_runtime.zig");
const number_runtime = @import("number_runtime.zig");
const gc = @import("gc_runtime_boehm.zig");

const c_allocator = std.heap.c_allocator;

// Codegen factories / helpers (emitted in every module).
extern fn zeus_new_JsonValue() *anyopaque;
extern fn zeus_new_u8_array(capacity: i32) *anyopaque;
extern fn zeus_new_string(data: *anyopaque) *anyopaque;
extern fn zeus_new_Object_array(capacity: i32) *anyopaque;
extern fn zeus_array_push(this_ptr: *anyopaque, return_buffer_ptr: ?*anyopaque, value_ptr: *anyopaque) void;

const K_NULL: i32 = 0;
const K_BOOL: i32 = 1;
const K_NUMBER: i32 = 2;
const K_STRING: i32 = 3;
const K_ARRAY: i32 = 4;
const K_OBJECT: i32 = 5;

// A JSON tree node. Pointer fields are GC-scanned (nodes/strings are GC-allocated), so the tree stays
// alive through JsonValue.node. Arrays (items / keys+vals) grow by doubling.
const JsonNode = struct {
    kind: i32,
    b: bool,
    num: f64,
    str: ?*anyopaque, // Zeus string object (string value)
    items: ?[*]?*JsonNode,
    items_len: usize,
    items_cap: usize,
    keys: ?[*]?*anyopaque, // Zeus string objects
    vals: ?[*]?*JsonNode,
    obj_len: usize,
    obj_cap: usize,
};

const JsonValueObj = extern struct {
    obj_header: *anyopaque,
    node: ?*JsonNode,
};

fn newNode(kind: i32) *JsonNode {
    const n: *JsonNode = @ptrCast(@alignCast(gc.zeus_gc_alloc(@sizeOf(JsonNode)).?));
    n.* = .{ .kind = kind, .b = false, .num = 0, .str = null, .items = null, .items_len = 0, .items_cap = 0, .keys = null, .vals = null, .obj_len = 0, .obj_cap = 0 };
    return n;
}

fn gcPtrArray(cap: usize) [*]?*anyopaque {
    return @ptrCast(@alignCast(gc.zeus_gc_alloc(@intCast(cap * @sizeOf(?*anyopaque))).?));
}

fn arrayPush(n: *JsonNode, child: *JsonNode) void {
    if (n.items_len >= n.items_cap) {
        const newcap = if (n.items_cap == 0) @as(usize, 4) else n.items_cap * 2;
        const mem: [*]?*JsonNode = @ptrCast(@alignCast(gcPtrArray(newcap)));
        if (n.items) |old| {
            var i: usize = 0;
            while (i < n.items_len) : (i += 1) mem[i] = old[i];
        }
        n.items = mem;
        n.items_cap = newcap;
    }
    n.items.?[n.items_len] = child;
    n.items_len += 1;
}

fn objectSet(n: *JsonNode, key: *anyopaque, val: *JsonNode) void {
    // Replace an existing key if present.
    var i: usize = 0;
    while (i < n.obj_len) : (i += 1) {
        if (keyEquals(n.keys.?[i], key)) {
            n.vals.?[i] = val;
            return;
        }
    }
    if (n.obj_len >= n.obj_cap) {
        const newcap = if (n.obj_cap == 0) @as(usize, 4) else n.obj_cap * 2;
        const kmem: [*]?*anyopaque = gcPtrArray(newcap);
        const vmem: [*]?*JsonNode = @ptrCast(@alignCast(gcPtrArray(newcap)));
        if (n.keys) |oldk| {
            var j: usize = 0;
            while (j < n.obj_len) : (j += 1) {
                kmem[j] = oldk[j];
                vmem[j] = n.vals.?[j];
            }
        }
        n.keys = kmem;
        n.vals = vmem;
        n.obj_cap = newcap;
    }
    n.keys.?[n.obj_len] = key;
    n.vals.?[n.obj_len] = val;
    n.obj_len += 1;
}

fn keyEquals(a: ?*anyopaque, b: ?*anyopaque) bool {
    return std.mem.eql(u8, string_runtime.zeusStringBytes(a), string_runtime.zeusStringBytes(b));
}

// Build a Zeus string object from raw bytes.
fn makeString(bytes: []const u8) *anyopaque {
    const arr_ptr = zeus_new_u8_array(@intCast(bytes.len));
    const arr = runtime_util.castToArrayObj(arr_ptr);
    if (bytes.len > 0) {
        if (arr.data) |dest| {
            @memcpy(@as([*]u8, @ptrCast(@alignCast(dest)))[0..bytes.len], bytes);
        }
    }
    arr.length = @intCast(bytes.len);
    return zeus_new_string(arr_ptr);
}

fn wrap(node: *JsonNode) *anyopaque {
    const v = zeus_new_JsonValue();
    const obj: *JsonValueObj = @ptrCast(@alignCast(v));
    obj.node = node;
    return v;
}

fn nodeOf(this_ptr: *anyopaque) ?*JsonNode {
    return @as(*JsonValueObj, @ptrCast(@alignCast(this_ptr))).node;
}

// ---- ABI helpers ----------------------------------------------------------

inline fn argObj(ptr_ptr: *anyopaque) *anyopaque {
    return @as(**anyopaque, @ptrCast(@alignCast(ptr_ptr))).*;
}
inline fn readF64(ptr: *anyopaque) f64 {
    return @as(*f64, @ptrCast(@alignCast(ptr))).*;
}
inline fn readBool(ptr: *anyopaque) bool {
    return @as(*bool, @ptrCast(@alignCast(ptr))).*;
}
inline fn readI32(ptr: *anyopaque) i32 {
    return @as(*i32, @ptrCast(@alignCast(ptr))).*;
}
fn retI32(rb: ?*anyopaque, v: i32) void {
    if (runtime_util.allocateReturnBuffer(rb, @sizeOf(i32))) |b| @as(*i32, @ptrCast(@alignCast(b.ptr))).* = v;
}
fn retBool(rb: ?*anyopaque, v: bool) void {
    if (runtime_util.allocateReturnBuffer(rb, @sizeOf(bool))) |b| b[0] = if (v) 1 else 0;
}
fn retF64(rb: ?*anyopaque, v: f64) void {
    if (runtime_util.allocateReturnBuffer(rb, @sizeOf(f64))) |b| @as(*f64, @ptrCast(@alignCast(b.ptr))).* = v;
}
fn retPtr(rb: ?*anyopaque, p: *anyopaque) void {
    if (runtime_util.allocateReturnBuffer(rb, @sizeOf(*anyopaque))) |b| @as(**anyopaque, @ptrCast(@alignCast(b.ptr))).* = p;
}

// ---- JsonValue accessors --------------------------------------------------

export fn zeus_JsonValue_kind(this_ptr: *anyopaque, rb: ?*anyopaque) callconv(.C) void {
    retI32(rb, if (nodeOf(this_ptr)) |n| n.kind else K_NULL);
}

fn isKind(this_ptr: *anyopaque, rb: ?*anyopaque, k: i32) void {
    retBool(rb, if (nodeOf(this_ptr)) |n| n.kind == k else (k == K_NULL));
}
export fn zeus_JsonValue_isNull(this_ptr: *anyopaque, rb: ?*anyopaque) callconv(.C) void {
    isKind(this_ptr, rb, K_NULL);
}
export fn zeus_JsonValue_isBool(this_ptr: *anyopaque, rb: ?*anyopaque) callconv(.C) void {
    isKind(this_ptr, rb, K_BOOL);
}
export fn zeus_JsonValue_isNumber(this_ptr: *anyopaque, rb: ?*anyopaque) callconv(.C) void {
    isKind(this_ptr, rb, K_NUMBER);
}
export fn zeus_JsonValue_isString(this_ptr: *anyopaque, rb: ?*anyopaque) callconv(.C) void {
    isKind(this_ptr, rb, K_STRING);
}
export fn zeus_JsonValue_isArray(this_ptr: *anyopaque, rb: ?*anyopaque) callconv(.C) void {
    isKind(this_ptr, rb, K_ARRAY);
}
export fn zeus_JsonValue_isObject(this_ptr: *anyopaque, rb: ?*anyopaque) callconv(.C) void {
    isKind(this_ptr, rb, K_OBJECT);
}

export fn zeus_JsonValue_asBool(this_ptr: *anyopaque, rb: ?*anyopaque) callconv(.C) void {
    retBool(rb, if (nodeOf(this_ptr)) |n| n.b else false);
}
export fn zeus_JsonValue_asNumber(this_ptr: *anyopaque, rb: ?*anyopaque) callconv(.C) void {
    retF64(rb, if (nodeOf(this_ptr)) |n| n.num else 0);
}
export fn zeus_JsonValue_asInt(this_ptr: *anyopaque, rb: ?*anyopaque) callconv(.C) void {
    const num = if (nodeOf(this_ptr)) |n| n.num else 0;
    if (std.math.isNan(num) or num > 2147483647.0 or num < -2147483648.0) return retI32(rb, 0);
    retI32(rb, @intFromFloat(@trunc(num)));
}
export fn zeus_JsonValue_asString(this_ptr: *anyopaque, rb: ?*anyopaque) callconv(.C) void {
    if (nodeOf(this_ptr)) |n| {
        if (n.str) |s| return retPtr(rb, s);
    }
    retPtr(rb, makeString(&[_]u8{}));
}
export fn zeus_JsonValue_length(this_ptr: *anyopaque, rb: ?*anyopaque) callconv(.C) void {
    retI32(rb, if (nodeOf(this_ptr)) |n| @intCast(n.items_len) else 0);
}
export fn zeus_JsonValue_at(this_ptr: *anyopaque, rb: ?*anyopaque, index_ptr: *anyopaque) callconv(.C) void {
    const idx = readI32(index_ptr);
    if (nodeOf(this_ptr)) |n| {
        if (idx >= 0 and @as(usize, @intCast(idx)) < n.items_len) {
            return retPtr(rb, wrap(n.items.?[@intCast(idx)].?));
        }
    }
    retPtr(rb, wrap(newNode(K_NULL)));
}
export fn zeus_JsonValue_has(this_ptr: *anyopaque, rb: ?*anyopaque, key_ptr: *anyopaque) callconv(.C) void {
    const key = argObj(key_ptr);
    if (nodeOf(this_ptr)) |n| {
        var i: usize = 0;
        while (i < n.obj_len) : (i += 1) {
            if (keyEquals(n.keys.?[i], key)) return retBool(rb, true);
        }
    }
    retBool(rb, false);
}
export fn zeus_JsonValue_get(this_ptr: *anyopaque, rb: ?*anyopaque, key_ptr: *anyopaque) callconv(.C) void {
    const key = argObj(key_ptr);
    if (nodeOf(this_ptr)) |n| {
        var i: usize = 0;
        while (i < n.obj_len) : (i += 1) {
            if (keyEquals(n.keys.?[i], key)) return retPtr(rb, wrap(n.vals.?[i].?));
        }
    }
    retPtr(rb, wrap(newNode(K_NULL)));
}
export fn zeus_JsonValue_objectKeys(this_ptr: *anyopaque, rb: ?*anyopaque) callconv(.C) void {
    const arr = zeus_new_Object_array(0);
    if (nodeOf(this_ptr)) |n| {
        var i: usize = 0;
        while (i < n.obj_len) : (i += 1) {
            var slot: ?*anyopaque = n.keys.?[i];
            zeus_array_push(arr, null, @ptrCast(&slot));
        }
    }
    retPtr(rb, arr);
}
export fn zeus_JsonValue_push(this_ptr: *anyopaque, rb: ?*anyopaque, value_ptr: *anyopaque) callconv(.C) void {
    _ = rb;
    if (nodeOf(this_ptr)) |n| {
        if (nodeOf(argObj(value_ptr))) |child| arrayPush(n, child);
    }
}
export fn zeus_JsonValue_set(this_ptr: *anyopaque, rb: ?*anyopaque, key_ptr: *anyopaque, value_ptr: *anyopaque) callconv(.C) void {
    _ = rb;
    if (nodeOf(this_ptr)) |n| {
        if (nodeOf(argObj(value_ptr))) |child| objectSet(n, argObj(key_ptr), child);
    }
}

// ---- JSON builders --------------------------------------------------------

export fn zeus_JSON_newObject(this_ptr: ?*anyopaque, rb: ?*anyopaque) callconv(.C) void {
    _ = this_ptr;
    retPtr(rb, wrap(newNode(K_OBJECT)));
}
export fn zeus_JSON_newArray(this_ptr: ?*anyopaque, rb: ?*anyopaque) callconv(.C) void {
    _ = this_ptr;
    retPtr(rb, wrap(newNode(K_ARRAY)));
}
export fn zeus_JSON_newString(this_ptr: ?*anyopaque, rb: ?*anyopaque, s_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    const n = newNode(K_STRING);
    n.str = argObj(s_ptr);
    retPtr(rb, wrap(n));
}
export fn zeus_JSON_newNumber(this_ptr: ?*anyopaque, rb: ?*anyopaque, n_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    const nd = newNode(K_NUMBER);
    nd.num = readF64(n_ptr);
    retPtr(rb, wrap(nd));
}
export fn zeus_JSON_newBool(this_ptr: ?*anyopaque, rb: ?*anyopaque, b_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    const nd = newNode(K_BOOL);
    nd.b = readBool(b_ptr);
    retPtr(rb, wrap(nd));
}
export fn zeus_JSON_newNull(this_ptr: ?*anyopaque, rb: ?*anyopaque) callconv(.C) void {
    _ = this_ptr;
    retPtr(rb, wrap(newNode(K_NULL)));
}

// ---- parse ----------------------------------------------------------------

const Parser = struct {
    src: []const u8,
    pos: usize,

    fn peek(self: *Parser) u8 {
        return if (self.pos < self.src.len) self.src[self.pos] else 0;
    }
    fn skipWs(self: *Parser) void {
        while (self.pos < self.src.len) : (self.pos += 1) {
            const c = self.src[self.pos];
            if (c != ' ' and c != '\t' and c != '\n' and c != '\r') break;
        }
    }
    fn hexVal(c: u8) u32 {
        if (c >= '0' and c <= '9') return c - '0';
        if (c >= 'a' and c <= 'f') return c - 'a' + 10;
        if (c >= 'A' and c <= 'F') return c - 'A' + 10;
        return 0;
    }
    fn parseHex4(self: *Parser) u32 {
        var cp: u32 = 0;
        var i: usize = 0;
        while (i < 4 and self.pos < self.src.len) : (i += 1) {
            cp = cp * 16 + hexVal(self.src[self.pos]);
            self.pos += 1;
        }
        return cp;
    }
    fn encodeUtf8(out: *std.ArrayList(u8), cp: u32) void {
        if (cp < 0x80) {
            out.append(@intCast(cp)) catch {};
        } else if (cp < 0x800) {
            out.append(@intCast(0xC0 | (cp >> 6))) catch {};
            out.append(@intCast(0x80 | (cp & 0x3F))) catch {};
        } else {
            out.append(@intCast(0xE0 | (cp >> 12))) catch {};
            out.append(@intCast(0x80 | ((cp >> 6) & 0x3F))) catch {};
            out.append(@intCast(0x80 | (cp & 0x3F))) catch {};
        }
    }
    // Returns a Zeus string object for the quoted string at the cursor.
    fn parseString(self: *Parser) *anyopaque {
        self.pos += 1; // opening quote
        var out = std.ArrayList(u8).init(c_allocator);
        defer out.deinit();
        while (self.pos < self.src.len) {
            const c = self.src[self.pos];
            self.pos += 1;
            if (c == '"') break;
            if (c == '\\' and self.pos < self.src.len) {
                const e = self.src[self.pos];
                self.pos += 1;
                switch (e) {
                    '"' => out.append('"') catch {},
                    '\\' => out.append('\\') catch {},
                    '/' => out.append('/') catch {},
                    'n' => out.append('\n') catch {},
                    'r' => out.append('\r') catch {},
                    't' => out.append('\t') catch {},
                    'b' => out.append(8) catch {},
                    'f' => out.append(12) catch {},
                    'u' => encodeUtf8(&out, self.parseHex4()),
                    else => out.append(e) catch {},
                }
            } else {
                out.append(c) catch {};
            }
        }
        return makeString(out.items);
    }
    fn parseNumber(self: *Parser) *JsonNode {
        const start = self.pos;
        while (self.pos < self.src.len) : (self.pos += 1) {
            const c = self.src[self.pos];
            if (!((c >= '0' and c <= '9') or c == '-' or c == '+' or c == '.' or c == 'e' or c == 'E')) break;
        }
        const n = newNode(K_NUMBER);
        n.num = std.fmt.parseFloat(f64, self.src[start..self.pos]) catch std.math.nan(f64);
        return n;
    }
    fn matchLiteral(self: *Parser, word: []const u8) void {
        self.pos += word.len;
    }
    fn parseValue(self: *Parser) *JsonNode {
        self.skipWs();
        const c = self.peek();
        switch (c) {
            '{' => return self.parseObject(),
            '[' => return self.parseArray(),
            '"' => {
                const n = newNode(K_STRING);
                n.str = self.parseString();
                return n;
            },
            't' => {
                self.matchLiteral("true");
                const n = newNode(K_BOOL);
                n.b = true;
                return n;
            },
            'f' => {
                self.matchLiteral("false");
                const n = newNode(K_BOOL);
                n.b = false;
                return n;
            },
            'n' => {
                self.matchLiteral("null");
                return newNode(K_NULL);
            },
            else => return self.parseNumber(),
        }
    }
    fn parseArray(self: *Parser) *JsonNode {
        const arr = newNode(K_ARRAY);
        self.pos += 1; // [
        self.skipWs();
        if (self.peek() == ']') {
            self.pos += 1;
            return arr;
        }
        while (true) {
            arrayPush(arr, self.parseValue());
            self.skipWs();
            const c = self.peek();
            self.pos += 1;
            if (c != ',') break; // ']' or malformed
        }
        return arr;
    }
    fn parseObject(self: *Parser) *JsonNode {
        const obj = newNode(K_OBJECT);
        self.pos += 1; // {
        self.skipWs();
        if (self.peek() == '}') {
            self.pos += 1;
            return obj;
        }
        while (true) {
            self.skipWs();
            const key = self.parseString();
            self.skipWs();
            if (self.peek() == ':') self.pos += 1;
            objectSet(obj, key, self.parseValue());
            self.skipWs();
            const c = self.peek();
            self.pos += 1;
            if (c != ',') break; // '}' or malformed
        }
        return obj;
    }
};

export fn zeus_JSON_parse(this_ptr: ?*anyopaque, rb: ?*anyopaque, text_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    var p = Parser{ .src = string_runtime.zeusStringBytes(argObj(text_ptr)), .pos = 0 };
    retPtr(rb, wrap(p.parseValue()));
}

// ---- stringify ------------------------------------------------------------

const hex_chars = "0123456789abcdef";

fn escapeInto(out: *std.ArrayList(u8), s: []const u8) void {
    out.append('"') catch {};
    for (s) |c| {
        switch (c) {
            '"' => out.appendSlice("\\\"") catch {},
            '\\' => out.appendSlice("\\\\") catch {},
            '\n' => out.appendSlice("\\n") catch {},
            '\r' => out.appendSlice("\\r") catch {},
            '\t' => out.appendSlice("\\t") catch {},
            8 => out.appendSlice("\\b") catch {},
            12 => out.appendSlice("\\f") catch {},
            else => {
                if (c < 32) {
                    out.appendSlice("\\u00") catch {};
                    out.append(hex_chars[(c >> 4) & 0x0F]) catch {};
                    out.append(hex_chars[c & 0x0F]) catch {};
                } else {
                    out.append(c) catch {};
                }
            },
        }
    }
    out.append('"') catch {};
}

fn stringifyNode(node: ?*JsonNode, out: *std.ArrayList(u8)) void {
    const n = node orelse {
        out.appendSlice("null") catch {};
        return;
    };
    switch (n.kind) {
        K_NULL => out.appendSlice("null") catch {},
        K_BOOL => out.appendSlice(if (n.b) "true" else "false") catch {},
        K_NUMBER => {
            if (!std.math.isFinite(n.num)) {
                out.appendSlice("null") catch {};
            } else {
                var buf: [64]u8 = undefined;
                out.appendSlice(number_runtime.formatFloat(f64, &buf, n.num)) catch {};
            }
        },
        K_STRING => escapeInto(out, string_runtime.zeusStringBytes(n.str)),
        K_ARRAY => {
            out.append('[') catch {};
            var i: usize = 0;
            while (i < n.items_len) : (i += 1) {
                if (i > 0) out.append(',') catch {};
                stringifyNode(n.items.?[i], out);
            }
            out.append(']') catch {};
        },
        K_OBJECT => {
            out.append('{') catch {};
            var i: usize = 0;
            while (i < n.obj_len) : (i += 1) {
                if (i > 0) out.append(',') catch {};
                escapeInto(out, string_runtime.zeusStringBytes(n.keys.?[i]));
                out.append(':') catch {};
                stringifyNode(n.vals.?[i], out);
            }
            out.append('}') catch {};
        },
        else => out.appendSlice("null") catch {},
    }
}

export fn zeus_JSON_stringify(this_ptr: ?*anyopaque, rb: ?*anyopaque, value_ptr: *anyopaque) callconv(.C) void {
    _ = this_ptr;
    var out = std.ArrayList(u8).init(c_allocator);
    defer out.deinit();
    stringifyNode(nodeOf(argObj(value_ptr)), &out);
    retPtr(rb, makeString(out.items));
}
