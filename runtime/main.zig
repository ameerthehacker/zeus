// Zeus Runtime Entry Point
// This file serves as the main entry point for the Zeus runtime,
// importing and coordinating all runtime modules.

const gc_runtime = @import("gc_runtime.zig");
const array_runtime = @import("array_runtime.zig");
const io_runtime = @import("io_runtime.zig");
const string_runtime = @import("string_runtime.zig");
const exception_runtime = @import("exception_runtime.zig");

// Ensure runtime modules are included by referencing them
comptime {
    _ = gc_runtime;
    _ = array_runtime;
    _ = io_runtime;
    _ = string_runtime;
    _ = exception_runtime;
}
