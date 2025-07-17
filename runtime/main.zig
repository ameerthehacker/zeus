const std = @import("std");

var gpa = std.heap.GeneralPurposeAllocator(.{}){};
const allocator = gpa.allocator();

// LLVM Statepoint Stack Map structures
const StackMapHeader = extern struct {
    version: u8,
    reserved1: u8,
    reserved2: u16,
    num_functions: u32,
    num_constants: u32,
    num_records: u32,
};

const StkSizeRecord = extern struct {
    function_address: u64,
    stack_size: u64,
    record_count: u64,
};

const LiveOut = extern struct {
    reg_num: u16,
    reserved: u8,
    size_in_bytes: u8,
};

const StackMapRecord = extern struct {
    patchpoint_id: u64,
    instruction_offset: u32,
    reserved: u16,
    num_locations: u16,
    // followed by Location entries and LiveOuts
};

const Location = extern struct {
    location_type: u8,
    reserved_1: u8,
    location_size: u16,
    dwarf_reg_num: u16,
    reserved_2: u16,
    offset_or_constant: i32,
};

// GC Root tracking
const GCRoot = struct {
    ptr: *anyopaque,
    size: u32,
    marked: bool,
};

var gc_roots = std.ArrayList(GCRoot).init(allocator);

// LLVM places the stack map in a special section
// We access it via linker symbols that LLVM generates
// Define as optional pointer to handle when symbol doesn't exist
var llvm_stackmaps_ptr: ?*StackMapHeader = null;
var symbol_lookup_attempted: bool = false;

fn isDebugEnabled() bool {
    const env_val = std.process.getEnvVarOwned(allocator, "ZEUS_GC_DEBUG") catch return false;
    defer allocator.free(env_val);
    return std.mem.eql(u8, env_val, "true");
}

fn log(comptime fmt: []const u8, args: anytype) void {
    if (!isDebugEnabled()) return;
    std.debug.print("zeus_gc: " ++ fmt ++ "\n", args);
}

// Track previous frame for stack movement analysis
var prev_frame_addr: usize = 0;
var safepoint_count: u32 = 0;

// Try to locate the LLVM stack map section at runtime
fn tryFindStackMapSymbol() ?*StackMapHeader {
    // On macOS/Darwin, access the section directly using mach-o APIs
    const c = @cImport({
        @cInclude("mach-o/getsect.h");
        @cInclude("mach-o/dyld.h");
    });

    // Get the main executable (index 0)
    const header = c._dyld_get_image_header(0);
    if (header == null) {
        log("  failed to get main executable header", .{});
        return null;
    }

    // Cast to mach_header_64 for ARM64/x86_64
    const header64 = @as([*c]const c.struct_mach_header_64, @ptrCast(header));

    // Find the __llvm_stackmaps section in the __LLVM_STACKMAPS segment (not __DATA!)
    var size: c_ulong = 0;
    const section_data = c.getsectiondata(header64, "__LLVM_STACKMAPS", "__llvm_stackmaps", &size);

    if (section_data == null or size == 0) {
        log("  __llvm_stackmaps section not found in __LLVM_STACKMAPS segment", .{});
        return null;
    }

    log("  found __llvm_stackmaps section: {} bytes at 0x{X}", .{ size, @intFromPtr(section_data) });

    return @as(*StackMapHeader, @ptrCast(@alignCast(section_data)));
}

// Access LLVM's stack map from the section
fn getStackMap() ?*StackMapHeader {
    // Try to locate the section if we haven't already attempted
    if (!symbol_lookup_attempted) {
        symbol_lookup_attempted = true;
        llvm_stackmaps_ptr = tryFindStackMapSymbol();

        if (llvm_stackmaps_ptr == null) {
            log("  __llvm_stackmaps section not found", .{});
        } else {
            log("  successfully accessed __llvm_stackmaps section!", .{});
        }
    }

    // Return cached result
    if (llvm_stackmaps_ptr == null) {
        return null;
    }

    const stack_map = llvm_stackmaps_ptr.?;
    const stack_map_addr = @intFromPtr(stack_map);
    log("  stack_map_address: 0x{X}", .{stack_map_addr});

    // Try to safely access the stack map header
    const version = blk: {
        // Use volatile read to prevent compiler optimizations
        const volatile_ptr = @as(*volatile StackMapHeader, stack_map);
        break :blk volatile_ptr.version;
    };

    log("  stack_map_version: {}", .{version});

    if (version == 0 or version > 10) {
        log("  invalid stack map version: {} (LLVM stack map not generated or corrupt)", .{version});
        return null;
    }

    log("  valid stack map found!", .{});
    return stack_map;
}

fn processStackMapAtSafepoint(_: usize) void {
    const stack_map = getStackMap() orelse {
        return; // Already logged in getStackMap()
    };

    log("=== STACK MAP INFO ===", .{});
    log("  version: {}", .{stack_map.version});
    log("  num_functions: {}", .{stack_map.num_functions});
    log("  num_records: {}", .{stack_map.num_records});

    // Find the record for this safepoint
    var record_ptr = @as([*]u8, @ptrCast(stack_map)) + @sizeOf(StackMapHeader);

    // Skip function records (each StkSizeRecord)
    log("  skipping {} function records...", .{stack_map.num_functions});
    for (0..stack_map.num_functions) |_| {
        record_ptr += @sizeOf(StkSizeRecord);
    }

    // Skip constants (each is u64)
    log("  skipping {} constant records...", .{stack_map.num_constants});
    record_ptr += stack_map.num_constants * @sizeOf(u64);

    log("  processing {} stack map records...", .{stack_map.num_records});

    // Process only the first record to avoid parsing issues
    const max_records_to_process = @min(stack_map.num_records, 1);
    for (0..max_records_to_process) |i| {
        // Ensure proper alignment for the record structure (8-byte aligned)
        const aligned_addr = std.mem.alignForward(usize, @intFromPtr(record_ptr), 8);
        record_ptr = @as([*]u8, @ptrFromInt(aligned_addr));

        const record = @as(*StackMapRecord, @ptrCast(@alignCast(record_ptr)));

        log("  Record {}: ID={}, offset=0x{X}, locations={}", .{ i, record.patchpoint_id, record.instruction_offset, record.num_locations });

        // Sanity check the record
        if (record.num_locations > 50) {
            log("    Record {} has suspicious location count: {}, skipping", .{ i, record.num_locations });
            continue;
        }

        // Process each location (these are your GC roots!)
        var loc_ptr = record_ptr + @sizeOf(StackMapRecord);
        for (0..record.num_locations) |j| {
            const location = @as(*Location, @ptrCast(@alignCast(loc_ptr)));

            const loc_type_name = switch (location.location_type) {
                1 => "Register",
                2 => "Direct",
                3 => "Indirect",
                4 => "Constant",
                5 => "ConstantIndex",
                else => "Unknown",
            };

            log("    Location {}: {s} (reg={}, offset={})", .{ j, loc_type_name, location.dwarf_reg_num, location.offset_or_constant });

            // If this is a pointer location, track it as a GC root
            if (location.location_type == 2 or location.location_type == 3) {
                trackGCRoot(location);
                log("      -> Tracking GC root: Direct/Indirect pointer!", .{});
            } else if (location.location_type == 4) {
                log("      -> Skipping constant value (not a GC root)", .{});
            }

            loc_ptr += @sizeOf(Location);
        }

        log("  successfully processed Record {}", .{i});

        // For now, only process the first record to avoid parsing complexity
        break;
    }
}

fn trackGCRoot(location: *Location) void {
    // Calculate actual address based on location info
    const frame_addr = @frameAddress();
    const root_addr = switch (location.location_type) {
        2 => frame_addr + @as(usize, @bitCast(@as(isize, location.offset_or_constant))), // Direct
        3 => blk: { // Indirect - read pointer from stack location
            const ptr_addr = frame_addr + @as(usize, @bitCast(@as(isize, location.offset_or_constant)));
            break :blk @as(*usize, @ptrFromInt(ptr_addr)).*;
        },
        else => return,
    };

    log("    -> Tracking GC root at 0x{X}", .{root_addr});

    // Add to our GC root set
    gc_roots.append(GCRoot{
        .ptr = @ptrFromInt(root_addr),
        .size = @as(u32, location.location_size),
        .marked = false,
    }) catch {};
}

fn performGarbageCollection() void {
    log("=== STARTING GC CYCLE ===", .{});
    log("  Tracked roots: {}", .{gc_roots.items.len});

    // Mark phase: mark all reachable objects
    for (gc_roots.items) |*root| {
        markObject(root);
    }

    // Sweep phase would go here...
    log("=== GC CYCLE COMPLETE ===", .{});

    // Clear roots for next cycle
    gc_roots.clearRetainingCapacity();
}

fn markObject(root: *GCRoot) void {
    if (root.marked) return;
    root.marked = true;
    log("  Marked object at 0x{X} (size={})", .{ @intFromPtr(root.ptr), root.size });

    // In a real GC, you'd traverse object references here
}

export fn gc_safepoint_slow_path() void {
    safepoint_count += 1;

    // Get current stack frame information
    const frame_addr = @frameAddress();
    const return_addr = @returnAddress();

    log("=== STATEPOINT #{} ===", .{safepoint_count});
    log("  frame_address: 0x{X}", .{frame_addr});
    log("  return_address: 0x{X}", .{return_addr});
    log("  attempting to access LLVM stack map...", .{});

    // Show stack movement since last safepoint
    if (prev_frame_addr != 0) {
        const stack_diff = if (frame_addr > prev_frame_addr)
            frame_addr - prev_frame_addr
        else
            prev_frame_addr - frame_addr;
        const direction = if (frame_addr > prev_frame_addr) "DOWN" else "UP";
        log("  stack_movement: {} bytes {s}", .{ stack_diff, direction });
    }

    // **NEW: Access LLVM's statepoint bookkeeping**
    processStackMapAtSafepoint(return_addr);

    // Trigger GC if we have enough roots
    if (gc_roots.items.len > 10) {
        performGarbageCollection();
    }

    prev_frame_addr = frame_addr;
}

export fn gc_alloc(size: u32) ?*anyopaque {
    const bytes = allocator.alloc(u8, size) catch return null;
    log("gc_alloc: {} bytes", .{size});
    return bytes.ptr;
}
