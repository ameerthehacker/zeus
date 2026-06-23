const gc_runtime = @import("gc_runtime_boehm.zig");
const array_runtime = @import("array_runtime.zig");
const io_runtime = @import("io_runtime.zig");
const string_runtime = @import("string_runtime.zig");
const exception_runtime = @import("exception_runtime.zig");

comptime {
    _ = gc_runtime;
    _ = array_runtime;
    _ = io_runtime;
    _ = string_runtime;
    _ = exception_runtime;
}
