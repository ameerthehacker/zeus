const gc_runtime = @import("gc_runtime_boehm.zig");
const array_runtime = @import("array_runtime.zig");
const io_runtime = @import("io_runtime.zig");
const string_runtime = @import("string_runtime.zig");
const number_runtime = @import("number_runtime.zig");
const exception_runtime = @import("exception_runtime.zig");
const timers_runtime = @import("timers_runtime.zig");
const math_runtime = @import("math_runtime.zig");
const c_ffi_runtime = @import("c_ffi_runtime.zig");
const process_runtime = @import("process_runtime.zig");

// Anchor the xev module import so Zig's module-declaration checker is satisfied.
// timers_runtime.zig is the actual consumer of xev.
pub const xev = @import("xev");

comptime {
    _ = gc_runtime;
    _ = array_runtime;
    _ = io_runtime;
    _ = string_runtime;
    _ = number_runtime;
    _ = exception_runtime;
    _ = timers_runtime;
    _ = math_runtime;
    _ = c_ffi_runtime;
    _ = process_runtime;
    _ = xev;
}
