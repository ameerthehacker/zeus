// Zeus Exception Handling Runtime
// Implements exception throwing, catching, and stack trace capture
// Uses libunwind for stack unwinding and setjmp/longjmp for control flow

const std = @import("std");
const abi = @import("abi.zig");
const colors = @import("colors.zig");

// C imports for libunwind, dladdr, and setjmp/longjmp
const c = @cImport({
    @cInclude("libunwind.h");
    @cInclude("dlfcn.h");
    @cInclude("setjmp.h");
});

// Use C allocator for exception structures
const allocator = std.heap.c_allocator;

/// Stack frame information for stack traces
pub const StackFrame = struct {
    instruction_pointer: usize,
    function_name: ?[]const u8,
    file_path: ?[]const u8,
    line: u32,
    column: u32,
};

/// Stack trace captured at throw time
pub const StackTrace = struct {
    frames: []StackFrame,
    alloc: std.mem.Allocator,

    pub fn deinit(self: *StackTrace) void {
        for (self.frames) |frame| {
            if (frame.function_name) |name| {
                self.alloc.free(name);
            }
            if (frame.file_path) |path| {
                self.alloc.free(path);
            }
        }
        self.alloc.free(self.frames);
    }
};

/// Zeus exception structure - custom format without C++ ABI
pub const ZeusException = struct {
    class_id: u32, // Zeus class ID for type matching
    object_ptr: *anyopaque, // Pointer to Error object instance
    stack_trace: ?*StackTrace, // Captured at throw time
    source_file: ?[*:0]const u8, // File where throw occurred
    source_line: u32, // Line number of throw
};

/// Exception handler information
const Handler = struct {
    class_ids: []const u32, // Class IDs this handler catches
    jmp_buf: *c.jmp_buf, // Jump buffer for longjmp
};

/// Thread-local exception state
threadlocal var current_exception: ?*ZeusException = null;

/// Handler stack for nested try blocks
threadlocal var handler_stack: std.ArrayList(Handler) = undefined;
threadlocal var handler_stack_initialized: bool = false;

/// Initialize handler stack (called once per thread)
fn ensureHandlerStackInit() void {
    if (!handler_stack_initialized) {
        handler_stack = std.ArrayList(Handler).init(allocator);
        handler_stack_initialized = true;
    }
}

/// Throw an exception - called from generated code
/// Uses longjmp to transfer control to the nearest matching handler
export fn zeus_throw(class_id: u32, object_ptr: *anyopaque, source_file: [*:0]const u8, source_line: u32) callconv(.C) noreturn {
    ensureHandlerStackInit();

    // Allocate and populate exception
    const exc = allocator.create(ZeusException) catch {
        @panic("Failed to allocate exception");
    };

    exc.* = ZeusException{
        .class_id = class_id,
        .object_ptr = object_ptr,
        .stack_trace = captureStackTrace(),
        .source_file = source_file,
        .source_line = source_line,
    };

    current_exception = exc;

    // Walk handler stack from top to find matching handler
    var i = handler_stack.items.len;
    while (i > 0) {
        i -= 1;
        const handler = handler_stack.items[i];

        // Check if any class ID in handler matches exception
        for (handler.class_ids) |handler_class_id| {
            if (checkClassHierarchy(class_id, object_ptr, handler_class_id)) {
                // Pop this handler (and inner ones unwound past) before jumping, so a re-throw in
                // the catch body goes to the next outer handler instead of looping on this one.
                handler_stack.shrinkRetainingCapacity(i);
                // longjmp transfers control to the setjmp call in zeus_try_begin
                c.longjmp(handler.jmp_buf, 1);
            }
        }
    }

    // No handler found - print stack trace and abort
    printUnhandledException(exc);
    std.process.exit(1);
}

/// Get current exception (for checking after calls)
export fn zeus_get_current_exception() callconv(.C) ?*ZeusException {
    return current_exception;
}

/// Get exception's class ID
export fn zeus_get_exception_class_id(exc: *ZeusException) callconv(.C) u32 {
    return exc.class_id;
}

/// Get the Error object pointer from exception
export fn zeus_get_exception_object(exc: *ZeusException) callconv(.C) *anyopaque {
    return exc.object_ptr;
}

/// Check if exception is instance of class (including subclasses)
export fn zeus_exception_instanceof(exc: *ZeusException, target_class_id: u32) callconv(.C) bool {
    // Get class hierarchy from object header and check inheritance chain
    return checkClassHierarchy(exc.class_id, exc.object_ptr, target_class_id);
}

/// Clear current exception (when caught)
export fn zeus_clear_exception() callconv(.C) void {
    if (current_exception) |exc| {
        // Free stack trace
        if (exc.stack_trace) |trace| {
            trace.deinit();
            allocator.destroy(trace);
        }
        allocator.destroy(exc);
    }
    current_exception = null;
}

/// Begin a try block - sets up setjmp and pushes handler
/// Returns 0 for normal execution (try body), 1 when exception is caught (catch body)
export fn zeus_try_begin(jmp_buf_ptr: *c.jmp_buf, class_ids_ptr: [*]const u32, num_classes: u32) callconv(.C) c_int {
    ensureHandlerStackInit();

    // First, push the handler onto the stack
    handler_stack.append(Handler{
        .class_ids = class_ids_ptr[0..num_classes],
        .jmp_buf = jmp_buf_ptr,
    }) catch @panic("Failed to push handler");

    // Call setjmp - returns 0 on initial call, 1 when longjmp is called
    return c.setjmp(jmp_buf_ptr);
}

/// Pop exception handler from stack (called at end of try block or catch block)
export fn zeus_pop_handler() callconv(.C) void {
    if (handler_stack_initialized and handler_stack.items.len > 0) {
        _ = handler_stack.pop();
    }
}

/// Unwind stack to find matching handler
/// Since we use a polling model (CHECK_EXCEPTION after each call),
/// we don't actually unwind - we just need to check if a handler exists.
/// The generated code will detect the exception and branch appropriately.
fn unwindToHandler() void {
    const exc = current_exception orelse return;

    // Walk handler stack from top to find matching handler
    var i = handler_stack.items.len;
    while (i > 0) {
        i -= 1;
        const handler = handler_stack.items[i];

        // Check if any class ID in handler matches exception
        for (handler.class_ids) |class_id| {
            if (checkClassHierarchy(exc.class_id, exc.object_ptr, class_id)) {
                // Found matching handler
                // The codegen inserts CHECK_EXCEPTION after each call which will detect this
                return;
            }
        }
    }

    // No handler found - print stack trace and abort
    printUnhandledException(exc);
    std.process.exit(1);
}

/// Check if class_id derives from target_class_id
fn checkClassHierarchy(class_id: u32, object_ptr: *anyopaque, target_class_id: u32) bool {
    if (class_id == target_class_id) return true;

    // Walk parent class chain using object header type info
    const obj = @as(*abi.ZeusObj, @ptrCast(@alignCast(object_ptr)));
    var type_info = obj.obj_header.getObjectTypeInfo();

    while (type_info.parent_type_info != null) {
        const parent_info = type_info.getParentTypeInfo();
        if (parent_info.object_type_id == target_class_id) {
            return true;
        }
        type_info = parent_info;
    }

    // Error base class (ID=1) catches all errors
    if (target_class_id == 1) return true;

    return false;
}

/// Runtime downcast / instanceof check used by `as` casts. Walks the object's class hierarchy
/// (via parent_type_info) looking for target_class_id, returning true if the object's dynamic
/// class is target_class_id or a subclass of it. Unlike checkClassHierarchy this has no
/// Error(id=1) catch-all, so it is a precise type test. A null object is not an instance of
/// anything. (bool/i1 return matches zeus_exception_instanceof's proven ABI shape.)
export fn zeus_instanceof(object_ptr: ?*anyopaque, target_class_id: u32) callconv(.C) bool {
    const ptr = object_ptr orelse return false;
    const obj = @as(*abi.ZeusObj, @ptrCast(@alignCast(ptr)));
    var type_info = obj.obj_header.getObjectTypeInfo();
    if (type_info.object_type_id == target_class_id) return true;
    while (type_info.parent_type_info != null) {
        const parent_info = type_info.getParentTypeInfo();
        if (parent_info.object_type_id == target_class_id) return true;
        type_info = parent_info;
    }
    return false;
}

/// Capture current stack trace using libunwind, dladdr, and DWARF debug info
fn captureStackTrace() ?*StackTrace {
    const trace = allocator.create(StackTrace) catch return null;
    var frames = std.ArrayList(StackFrame).init(allocator);

    // Get debug info for DWARF line number lookup
    const debug_info = std.debug.getSelfDebugInfo() catch null;

    var cursor: c.unw_cursor_t = undefined;
    var context: c.unw_context_t = undefined;

    // Get current context
    const ctx_result = c.unw_getcontext(&context);
    if (ctx_result != 0) {
        trace.* = StackTrace{
            .frames = frames.toOwnedSlice() catch &[_]StackFrame{},
            .alloc = allocator,
        };
        return trace;
    }

    // Initialize cursor
    const init_result = c.unw_init_local(&cursor, &context);
    if (init_result != 0) {
        trace.* = StackTrace{
            .frames = frames.toOwnedSlice() catch &[_]StackFrame{},
            .alloc = allocator,
        };
        return trace;
    }

    // Walk the stack, skipping internal frames
    const max_frames: u32 = 50;
    var frame_count: u32 = 0;
    var should_capture: bool = false;

    while (frame_count < max_frames) : (frame_count += 1) {
        // Get instruction pointer
        var ip: c.unw_word_t = 0;
        const reg_result = c.unw_get_reg(&cursor, c.UNW_REG_IP, &ip);
        if (reg_result != 0) {
            break;
        }

        if (ip == 0) {
            break;
        }

        // Subtract 1 from IP to get the actual call instruction (not return address)
        const lookup_ip: usize = if (ip > 0) @intCast(ip - 1) else @intCast(ip);

        // Get function name using dladdr
        var info: c.Dl_info = undefined;
        var func_name: ?[]const u8 = null;
        var file_path: ?[]const u8 = null;
        var line_num: u32 = 0;

        if (c.dladdr(@ptrFromInt(lookup_ip), &info) != 0) {
            // Get function name
            if (info.dli_sname != null) {
                const name_len = std.mem.len(info.dli_sname);
                if (name_len > 0) {
                    // Skip internal Zeus runtime frames (functions starting with zeus_)
                    const name_slice = info.dli_sname[0..name_len];
                    if (std.mem.startsWith(u8, name_slice, "zeus_") or
                        std.mem.startsWith(u8, name_slice, "captureStackTrace") or
                        std.mem.startsWith(u8, name_slice, "unwindToHandler") or
                        std.mem.startsWith(u8, name_slice, "printUnhandledException"))
                    {
                        // Skip this frame - move to next
                        const step_result = c.unw_step(&cursor);
                        if (step_result <= 0) break;
                        continue;
                    }

                    // Start capturing after we see a non-internal frame
                    should_capture = true;

                    const name_buf = allocator.alloc(u8, name_len) catch null;
                    if (name_buf) |buf| {
                        @memcpy(buf, info.dli_sname[0..name_len]);
                        func_name = buf;
                    }
                }
            }

        }

        // Try to get source location from DWARF debug info using adjusted IP
        if (debug_info) |di| {
            if (di.getModuleForAddress(lookup_ip)) |module| {
                const symbol = module.getSymbolAtAddress(allocator, lookup_ip) catch null;
                if (symbol) |sym| {
                    if (sym.line_info) |loc| {
                        if (loc.file_name.len > 0) {
                            const path_buf = allocator.alloc(u8, loc.file_name.len) catch null;
                            if (path_buf) |buf| {
                                @memcpy(buf, loc.file_name);
                                file_path = buf;
                            }
                        }
                        line_num = @intCast(loc.line);
                    }
                }
            } else |_| {}
        }

        // Only capture if we've passed internal frames
        if (!should_capture) {
            const step_result = c.unw_step(&cursor);
            if (step_result <= 0) break;
            continue;
        }

        frames.append(StackFrame{
            .instruction_pointer = @intCast(ip),
            .function_name = func_name,
            .file_path = file_path,
            .line = line_num,
            .column = 0,
        }) catch break;

        // Move to next frame
        const step_result = c.unw_step(&cursor);
        if (step_result <= 0) break;
    }

    trace.* = StackTrace{
        .frames = frames.toOwnedSlice() catch &[_]StackFrame{},
        .alloc = allocator,
    };

    return trace;
}

/// Print unhandled exception with full stack trace
fn printUnhandledException(exc: *const ZeusException) void {
    const stderr = std.io.getStdErr().writer();

    // Print exception header with error name
    stderr.print("\n{s}Unhandled Exception{s}: ", .{ colors.get(colors.red_bold), colors.get(colors.reset) }) catch {};

    // Print error name and message
    printErrorNameAndMessage(stderr, exc.object_ptr);

    stderr.print("\n\n{s}Stack Trace:{s}\n", .{ colors.get(colors.bold), colors.get(colors.reset) }) catch {};

    // Print each frame with source location and code snippet
    if (exc.stack_trace) |trace| {
        for (trace.frames, 0..) |frame, i| {
            const func_name = frame.function_name orelse "<unknown>";

            stderr.print("  {d}: {s}{s}{s}\n", .{ i, colors.get(colors.yellow), func_name, colors.get(colors.reset) }) catch {};

            // Print source location if available
            if (frame.file_path) |file_path| {
                if (frame.line > 0) {
                    stderr.print("       at {s}:{d}\n", .{ file_path, frame.line }) catch {};
                    // Print code snippet
                    printCodeSnippetIndented(stderr, file_path, frame.line);
                } else {
                    stderr.print("       at {s}\n", .{file_path}) catch {};
                }
            } else {
                // Fallback to address if no source info
                stderr.print("       at 0x{x}\n", .{frame.instruction_pointer}) catch {};
            }
        }
    } else {
        stderr.print("  (no stack trace available)\n", .{}) catch {};
    }

    stderr.print("\n", .{}) catch {};
}

/// Print code snippet with extra indentation for stack frames
fn printCodeSnippetIndented(writer: anytype, file_path: []const u8, line: u32) void {
    if (line == 0) return;

    const file = std.fs.cwd().openFile(file_path, .{}) catch return;
    defer file.close();

    var reader = file.reader();
    var line_num: u32 = 1;
    var buf: [1024]u8 = undefined;

    // Print context: line before, error line, line after
    while (reader.readUntilDelimiterOrEof(&buf, '\n') catch null) |line_content| {
        if (line_num >= line -| 1 and line_num <= line + 1) {
            if (line_num == line) {
                writer.print("       {s}>{s} {d:>4} | {s}{s}{s}\n", .{
                    colors.get(colors.red), colors.get(colors.reset),
                    line_num,
                    colors.get(colors.red), line_content, colors.get(colors.reset),
                }) catch {};
            } else {
                writer.print("         {d:>4} | {s}\n", .{ line_num, line_content }) catch {};
            }
        }

        if (line_num > line + 1) break;
        line_num += 1;
    }
}

/// Print code snippet around the error line
fn printCodeSnippet(writer: anytype, file_path: []const u8, line: u32) void {
    if (line == 0) return;

    const file = std.fs.cwd().openFile(file_path, .{}) catch return;
    defer file.close();

    var reader = file.reader();
    var line_num: u32 = 1;
    var buf: [1024]u8 = undefined;

    // Print context: line before, error line, line after
    while (reader.readUntilDelimiterOrEof(&buf, '\n') catch null) |line_content| {
        if (line_num >= line -| 1 and line_num <= line + 1) {
            if (line_num == line) {
                writer.print("  {s}>{s} {d:>4} | {s}{s}{s}\n", .{
                    colors.get(colors.red), colors.get(colors.reset),
                    line_num,
                    colors.get(colors.red), line_content, colors.get(colors.reset),
                }) catch {};
            } else {
                writer.print("    {d:>4} | {s}\n", .{ line_num, line_content }) catch {};
            }
        }

        if (line_num > line + 1) break;
        line_num += 1;
    }
}

/// Extract string content from a Zeus string object
fn extractStringContent(str_ptr: ?*anyopaque) ?[]const u8 {
    if (str_ptr) |ptr| {
        const string_obj = @as(*StringObj, @ptrCast(@alignCast(ptr)));
        if (string_obj.data) |array| {
            if (array.length > 0 and array.data != null) {
                return @as([*]const u8, @ptrCast(@alignCast(array.data.?)))[0..@intCast(array.length)];
            }
        }
    }
    return null;
}

/// Print error name and message from Error object
/// Format: "ErrorName: error message"
fn printErrorNameAndMessage(writer: anytype, obj_ptr: *anyopaque) void {
    const error_obj = castToErrorObj(obj_ptr);

    // Print error name
    if (extractStringContent(error_obj.name)) |name_bytes| {
        writer.print("{s}{s}{s}", .{ colors.get(colors.yellow), name_bytes, colors.get(colors.reset) }) catch {};
    } else {
        writer.print("{s}Error{s}", .{ colors.get(colors.yellow), colors.get(colors.reset) }) catch {};
    }

    // Print separator and message
    writer.print(": ", .{}) catch {};

    if (extractStringContent(error_obj.message)) |msg_bytes| {
        writer.print("{s}", .{msg_bytes}) catch {};
    } else {
        writer.print("(no message)", .{}) catch {};
    }
}

/// Zeus string object structure
const StringObj = extern struct {
    obj_header: *abi.ZeusObjectHeader,
    data: ?*abi.ZeusArrayObj,
    length: i32,
};

/// Zeus Error object structure
/// Must match the layout generated by the compiler for the Error primordial class
/// Properties are in order: name, message
pub const ZeusErrorObj = extern struct {
    obj_header: *abi.ZeusObjectHeader,
    name: ?*anyopaque, // Pointer to string object containing error name (e.g., "IndexOutOfBoundsException")
    message: ?*anyopaque, // Pointer to string object containing error message
};

/// Cast raw pointer to ZeusErrorObj
pub fn castToErrorObj(ptr: *anyopaque) *ZeusErrorObj {
    return @as(*ZeusErrorObj, @ptrCast(@alignCast(ptr)));
}

/// Error constructor: zeus_error_constructor(this_ptr, return_buffer, name_ptr_ptr, message_ptr_ptr)
/// Called when `new Error("name", "message")` is executed
/// @param this_ptr - Pointer to the newly allocated Error object
/// @param return_buffer_ptr - Not used for void return
/// @param name_ptr_ptr - Pointer to pointer to string object containing the error name
/// @param message_ptr_ptr - Pointer to pointer to string object containing the error message
export fn zeus_error_constructor(this_ptr: *anyopaque, return_buffer_ptr: ?*anyopaque, name_ptr_ptr: *anyopaque, message_ptr_ptr: *anyopaque) callconv(.C) void {
    _ = return_buffer_ptr;

    const error_obj = castToErrorObj(this_ptr);

    // Dereference to get the string object pointers
    const name_obj_ptr = @as(**anyopaque, @ptrCast(@alignCast(name_ptr_ptr))).*;
    const message_obj_ptr = @as(**anyopaque, @ptrCast(@alignCast(message_ptr_ptr))).*;

    // Store the name and message in the error object
    error_obj.name = name_obj_ptr;
    error_obj.message = message_obj_ptr;
}
