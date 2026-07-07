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
    id: i32,
    timer: xev.Timer,
    completion: xev.Completion,
    functor_ptr: *anyopaque, // direct GC-heap pointer to the functor object
    interval_ms: u64, // 0 for setTimeout, >0 for setInterval
    cancelled: bool,
    next: ?*TimerNode,
};

var global_loop: xev.Loop = undefined;
var loop_initialized: bool = false;
var pending_count: u32 = 0;
var timer_list_head: ?*TimerNode = null;
var next_timer_id: i32 = 1;

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

fn cancelTimerById(id: i32) void {
    var node = timer_list_head;
    while (node) |n| {
        if (n.id == id) n.cancelled = true;
        node = n.next;
    }
}

fn timerCallback(
    node: ?*TimerNode,
    _: *xev.Loop,
    _: *xev.Completion,
    r: xev.Timer.RunError!void,
) xev.CallbackAction {
    _ = r catch {};
    if (node) |n| {
        if (n.cancelled) {
            if (pending_count > 0) pending_count -= 1;
            return .disarm;
        }
        runtime_util.callFunctor(n.functor_ptr);
        // Re-check cancelled: the callback itself may have called clearInterval.
        if (!n.cancelled and n.interval_ms > 0) {
            // Reschedule: allocate a fresh node so the old completion can be disarmed.
            const raw = gc.zeus_gc_alloc(@sizeOf(TimerNode)) orelse @panic("zeus: OOM rescheduling interval");
            const new_node = @as(*TimerNode, @ptrCast(@alignCast(raw)));
            new_node.id = n.id;
            new_node.functor_ptr = n.functor_ptr;
            new_node.interval_ms = n.interval_ms;
            new_node.cancelled = false;
            new_node.next = timer_list_head;
            timer_list_head = new_node;
            new_node.timer = xev.Timer.init() catch @panic("zeus: timer re-init failed");
            new_node.timer.run(&global_loop, &new_node.completion, n.interval_ms, TimerNode, new_node, timerCallback);
            // pending_count unchanged: old removed, new added.
        } else {
            if (pending_count > 0) pending_count -= 1;
        }
    }
    return .disarm;
}

fn scheduleTimer(
    return_buffer_ptr: ?*anyopaque,
    callback_ptr_ptr: *anyopaque,
    delay_ptr_ptr: *anyopaque,
    is_interval: bool,
) void {
    // Dereference the ptr_ptr NOW while the LLVM wrapper's stack frame is still alive.
    const functor_ptr = @as(**anyopaque, @ptrCast(@alignCast(callback_ptr_ptr))).*;
    const delay_i32 = @as(*i32, @ptrCast(@alignCast(delay_ptr_ptr))).*;
    const delay_ms: u64 = @intCast(@max(0, delay_i32));
    // setInterval enforces a minimum 1ms repeat to prevent tight busy-loops.
    const interval_ms: u64 = if (is_interval) @max(1, delay_ms) else 0;

    ensureLoop();

    const raw = gc.zeus_gc_alloc(@sizeOf(TimerNode)) orelse @panic("zeus: OOM allocating timer node");
    const node = @as(*TimerNode, @ptrCast(@alignCast(raw)));
    node.id = next_timer_id;
    node.functor_ptr = functor_ptr;
    node.interval_ms = interval_ms;
    node.cancelled = false;
    node.next = timer_list_head;
    timer_list_head = node;

    const id = next_timer_id;
    next_timer_id += 1;
    pending_count += 1;

    node.timer = xev.Timer.init() catch @panic("zeus: xev.Timer.init failed");
    node.timer.run(&global_loop, &node.completion, delay_ms, TimerNode, node, timerCallback);

    if (return_buffer_ptr) |rb| {
        @as(*i32, @ptrCast(@alignCast(rb))).* = id;
    }
}

export fn zeus_setTimeout(
    return_buffer_ptr: ?*anyopaque,
    callback_ptr_ptr: *anyopaque,
    delay_ptr_ptr: *anyopaque,
) callconv(.C) void {
    scheduleTimer(return_buffer_ptr, callback_ptr_ptr, delay_ptr_ptr, false);
}

export fn zeus_setInterval(
    return_buffer_ptr: ?*anyopaque,
    callback_ptr_ptr: *anyopaque,
    delay_ptr_ptr: *anyopaque,
) callconv(.C) void {
    scheduleTimer(return_buffer_ptr, callback_ptr_ptr, delay_ptr_ptr, true);
}

export fn zeus_clearTimeout(
    return_buffer_ptr: ?*anyopaque,
    id_ptr: *anyopaque,
) callconv(.C) void {
    _ = return_buffer_ptr;
    cancelTimerById(@as(*i32, @ptrCast(@alignCast(id_ptr))).*);
}

export fn zeus_clearInterval(
    return_buffer_ptr: ?*anyopaque,
    id_ptr: *anyopaque,
) callconv(.C) void {
    _ = return_buffer_ptr;
    cancelTimerById(@as(*i32, @ptrCast(@alignCast(id_ptr))).*);
}
