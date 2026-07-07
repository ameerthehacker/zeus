const std = @import("std");
const xev = @import("xev");
const runtime_util = @import("runtime_util.zig");
const gc = @import("gc_runtime_boehm.zig");

const c = @cImport(@cInclude("stdlib.h"));

// Each pending timer is stored as a GC-allocated node in a linked list rooted
// at timer_list_head.  Because timer_list_head is a module-level (BSS) variable
// the Boehm GC always scans it as a root, keeping every TimerNode — including
// the embedded xev.Completion and functor pointer — alive until the callback fires.
//
// We store functor_ptr (the direct heap pointer to the functor object), NOT the
// callback_ptr_ptr passed to zeus_setTimeout.  The ptr_ptr convention uses a
// stack alloca inside the LLVM wrapper function; that alloca is gone by the time
// the atexit event loop drains, so we must dereference it immediately at call time.
const TimerNode = struct {
    timer: xev.Timer,
    completion: xev.Completion,
    functor_ptr: *anyopaque, // direct GC-heap pointer to the functor object
    next: ?*TimerNode,
};

var global_loop: xev.Loop = undefined;
var loop_initialized: bool = false;
var pending_count: u32 = 0;
var timer_list_head: ?*TimerNode = null;

fn ensureLoop() void {
    if (loop_initialized) return;
    global_loop = xev.Loop.init(.{}) catch @panic("zeus: event loop init failed");
    loop_initialized = true;
    _ = c.atexit(&drainEventLoop);
}

// Called by the C runtime after Zeus main() returns via atexit().
fn drainEventLoop() callconv(.C) void {
    if (!loop_initialized or pending_count == 0) return;
    global_loop.run(.until_done) catch {};
}

fn timerCallback(
    node: ?*TimerNode,
    _: *xev.Loop,
    _: *xev.Completion,
    r: xev.Timer.RunError!void,
) xev.CallbackAction {
    _ = r catch {};
    if (node) |n| {
        runtime_util.callFunctor(n.functor_ptr);
    }
    if (pending_count > 0) pending_count -= 1;
    return .disarm;
}

/// zeus_setTimeout: schedule a callback after delay milliseconds.
/// callback_ptr_ptr: ptr_ptr to the () => void callback functor
/// delay_ptr_ptr:    ptr_ptr to the i32 delay in milliseconds
export fn zeus_setTimeout(
    return_buffer_ptr: ?*anyopaque,
    callback_ptr_ptr: *anyopaque,
    delay_ptr_ptr: *anyopaque,
) callconv(.C) void {
    _ = return_buffer_ptr;
    const delay_i32 = @as(*i32, @ptrCast(@alignCast(delay_ptr_ptr))).*;
    const delay_ms: u64 = @intCast(@max(0, delay_i32));

    ensureLoop();

    // Dereference the ptr_ptr NOW while the LLVM wrapper's stack frame is still alive.
    // The wrapper stores the functor pointer in a stack alloca; after it returns that
    // slot is gone.  We pin the functor directly in the GC-allocated node.
    const functor_ptr = @as(**anyopaque, @ptrCast(@alignCast(callback_ptr_ptr))).*;

    // GC-allocate the node; store it at the head of timer_list_head so the
    // Boehm GC can always reach it and won't reclaim the embedded Completion.
    const raw = gc.zeus_gc_alloc(@sizeOf(TimerNode)) orelse @panic("zeus: OOM allocating timer node");
    const node = @as(*TimerNode, @ptrCast(@alignCast(raw)));
    node.functor_ptr = functor_ptr;
    node.next = timer_list_head;
    timer_list_head = node;

    node.timer = xev.Timer.init() catch @panic("zeus: xev.Timer.init failed");
    pending_count += 1;
    node.timer.run(&global_loop, &node.completion, delay_ms, TimerNode, node, timerCallback);
}
