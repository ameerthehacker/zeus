const std = @import("std");
const types = @import("stackmap_types.zig");

// Import the stackmap and GC types
const StackMapHeader = types.StackMapHeader;
const StackSizeRecord = types.StackSizeRecord;
const LiveOut = types.LiveOut;
const StackMapRecord = types.StackMapRecord;
const Location = types.Location;
// Structure to track GC roots - just the pointer and whether it's marked
const GCRoot = struct { ptr: *anyopaque, marked: bool };

// Structure to track allocated objects
const AllocatedObject = struct { ptr: *anyopaque, size: u32, marked: bool };

var gpa = std.heap.GeneralPurposeAllocator(.{}){};
const allocator = gpa.allocator();

var gc_roots = std.ArrayList(GCRoot).init(allocator);
// Global tracking of all allocated objects
var allocated_objects = std.ArrayList(AllocatedObject).init(allocator);
// Mutex to protect allocated_objects list during concurrent access
var alloc_mutex = std.Thread.Mutex{};

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
    std.io.getStdOut().writer().print("zeus_gc: " ++ fmt ++ "\n", args) catch {};
}

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
        std.debug.panic("failed to get main executable header", .{});
    }

    // Cast to mach_header_64 for ARM64/x86_64
    const header64 = @as([*c]const c.struct_mach_header_64, @ptrCast(header));

    // Find the __llvm_stackmaps section in the __LLVM_STACKMAPS segment (not __DATA!)
    var size: c_ulong = 0;
    const section_data = c.getsectiondata(header64, "__LLVM_STACKMAPS", "__llvm_stackmaps", &size);

    if (section_data == null or size == 0) {
        std.debug.panic("__llvm_stackmaps section not found in __LLVM_STACKMAPS segment", .{});
    }

    return @as(*StackMapHeader, @ptrCast(@alignCast(section_data)));
}

// Access LLVM's stack map from the section
fn getStackMap() ?*StackMapHeader {
    // Try to locate the section if we haven't already attempted
    if (!symbol_lookup_attempted) {
        symbol_lookup_attempted = true;
        llvm_stackmaps_ptr = tryFindStackMapSymbol();

        if (llvm_stackmaps_ptr == null) {
            std.debug.panic("__llvm_stackmaps section not found", .{});
        }
    }

    // Return cached result
    if (llvm_stackmaps_ptr == null) {
        return null;
    }

    const stack_map = llvm_stackmaps_ptr.?;

    // Try to safely access the stack map header
    const version = blk: {
        // Use volatile read to prevent compiler optimizations
        const volatile_ptr = @as(*volatile StackMapHeader, stack_map);
        break :blk volatile_ptr.version;
    };

    if (version == 0 or version > 10) {
        std.debug.panic("invalid stack map version: {} (LLVM stack map not generated or corrupt)", .{version});
        return null;
    }

    return stack_map;
}

fn processStackMapAtSafepoint(return_addr: usize, caller_frame_addr: usize) void {
    const stack_map = getStackMap().?;

    // Find the record for this safepoint
    var record_ptr = @as([*]u8, @ptrCast(stack_map)) + @sizeOf(StackMapHeader);

    // Find which function our return address belongs to and track function info
    var function_start_addr: u64 = 0;
    var target_function_index: ?u32 = null;
    var records_to_skip: u64 = 0;
    var target_function_record_count: u64 = 0;

    for (0..stack_map.num_functions) |func_idx| {
        const func_record = @as(*StackSizeRecord, @ptrCast(@alignCast(record_ptr)));
        if (return_addr >= func_record.function_address) {
            // This could be our function, but keep looking for a better match
            function_start_addr = func_record.function_address;
            target_function_index = @as(u32, @intCast(func_idx));
            target_function_record_count = func_record.record_count;
            // Don't update records_to_skip yet - we'll calculate it after we find the best match
        } else if (target_function_index != null) {
            // We found a function address that's higher than our return address,
            // so the previous function was our target
            break;
        }
        record_ptr += @sizeOf(StackSizeRecord);
    }

    // If we didn't find a matching function, bail out
    if (target_function_index == null) {
        log("gc_safepoint: no matching function found for return address 0x{X}", .{return_addr});
        return;
    }

    // Calculate how many records we need to skip to get to our function's records
    record_ptr = @as([*]u8, @ptrCast(stack_map)) + @sizeOf(StackMapHeader);
    for (0..target_function_index.?) |_| {
        const func_record = @as(*StackSizeRecord, @ptrCast(@alignCast(record_ptr)));
        records_to_skip += func_record.record_count;
        record_ptr += @sizeOf(StackSizeRecord);
    }

    // Skip constants
    record_ptr += stack_map.num_constants * @sizeOf(u64);

    // Calculate instruction offset from return address
    const instruction_offset = if (return_addr > function_start_addr)
        @as(u32, @intCast(return_addr - function_start_addr))
    else
        0;

    log("gc_safepoint: looking for instruction_offset: {} in function {} (skipping {} records, searching {} records)", .{ instruction_offset, target_function_index.?, records_to_skip, target_function_record_count });

    // Skip to the start of our function's records
    for (0..records_to_skip) |_| {
        // Ensure proper alignment for the record structure (8-byte aligned)
        const aligned_addr = std.mem.alignForward(usize, @intFromPtr(record_ptr), 8);
        record_ptr = @as([*]u8, @ptrFromInt(aligned_addr));

        const record = @as(*StackMapRecord, @ptrCast(@alignCast(record_ptr)));

        // Skip to next record
        var ptr = record_ptr + @sizeOf(StackMapRecord);

        // Skip locations
        ptr += record.num_locations * @sizeOf(Location);

        // Align to 8-byte boundary after locations
        ptr = @as([*]u8, @ptrFromInt(std.mem.alignForward(usize, @intFromPtr(ptr), 8)));

        // Read and skip LiveOuts
        const num_liveouts = @as(*u16, @ptrCast(@alignCast(ptr))).*;
        ptr += @sizeOf(u16);
        ptr += num_liveouts * @sizeOf(LiveOut);

        // Align to 8-byte boundary for next record
        record_ptr = @as([*]u8, @ptrFromInt(std.mem.alignForward(usize, @intFromPtr(ptr), 8)));
    }

    // Find the exact statepoint record that matches our return address within our function's records
    for (0..target_function_record_count) |_| {
        // Ensure proper alignment for the record structure (8-byte aligned)
        const aligned_addr = std.mem.alignForward(usize, @intFromPtr(record_ptr), 8);
        record_ptr = @as([*]u8, @ptrFromInt(aligned_addr));

        const record = @as(*StackMapRecord, @ptrCast(@alignCast(record_ptr)));

        // Check if this is the exact statepoint record we're looking for
        const offset_match = (record.instruction_offset == instruction_offset);
        const is_statepoint = record.patchpoint_id == 2882400000;

        if (offset_match and is_statepoint) {
            log("gc_safepoint: found matching statepoint record at offset {}", .{instruction_offset});
            processStatemapRecord(record, caller_frame_addr);
            break; // We found our record, no need to continue
        }

        // Skip to next record
        var ptr = record_ptr + @sizeOf(StackMapRecord);

        // Skip locations
        ptr += record.num_locations * @sizeOf(Location);

        // Align to 8-byte boundary after locations
        ptr = @as([*]u8, @ptrFromInt(std.mem.alignForward(usize, @intFromPtr(ptr), 8)));

        // Read and skip LiveOuts
        const num_liveouts = @as(*u16, @ptrCast(@alignCast(ptr))).*;
        ptr += @sizeOf(u16);
        ptr += num_liveouts * @sizeOf(LiveOut);

        // Align to 8-byte boundary for next record
        record_ptr = @as([*]u8, @ptrFromInt(std.mem.alignForward(usize, @intFromPtr(ptr), 8)));
    }
}

fn processStatemapRecord(record: *StackMapRecord, caller_frame_addr: usize) void {
    var ptr = @as([*]u8, @ptrCast(record)) + @sizeOf(StackMapRecord);

    if (record.num_locations < 3) {
        return; // Not enough locations for a valid statepoint
    }

    // Skip calling convention
    ptr += @sizeOf(Location);

    // Skip flags
    ptr += @sizeOf(Location);

    // Get number of deopt arguments
    const num_deopt_args = @as(u32, @intCast(@as(*Location, @ptrCast(@alignCast(ptr))).offset_or_constant));
    ptr += @sizeOf(Location);

    // Skip deopt arguments
    for (0..num_deopt_args) |_| {
        ptr += @sizeOf(Location);
    }

    // Calculate remaining locations (should be base/derived pointer pairs)
    const remaining_locations = record.num_locations - 3 - num_deopt_args;
    log("gc_safepoint: remaining_locations: {}", .{remaining_locations});

    if (remaining_locations > 0 and remaining_locations % 2 == 0) {
        const num_relocation_pairs = remaining_locations / 2;
        log("gc_safepoint: num_relocation_pairs: {}", .{num_relocation_pairs});

        for (0..num_relocation_pairs) |_| {
            // Base pointer location
            const base_location = @as(*Location, @ptrCast(@alignCast(ptr)));
            ptr += @sizeOf(Location);

            // Derived pointer location
            const derived_location = @as(*Location, @ptrCast(@alignCast(ptr)));
            ptr += @sizeOf(Location);

            // Track GC roots from stack locations
            trackGCRoot(base_location, caller_frame_addr);
            trackGCRoot(derived_location, caller_frame_addr);
        }
    }
}

fn trackGCRoot(location: *Location, frame_addr: usize) void {
    // Only process Direct and Indirect locations that contain pointers
    if (location.location_type != 2 and location.location_type != 3) {
        return;
    }

    // Skip if location size is not pointer-sized
    if (location.location_size != @sizeOf(*anyopaque)) {
        return;
    }

    const stack_location_addr = blk: {
        const offset = @as(isize, location.offset_or_constant);
        break :blk frame_addr + @as(usize, @bitCast(offset));
    };

    // Read the pointer value from the stack location
    // +16 to skip the return address
    const pointer_value = @as(*usize, @ptrFromInt(stack_location_addr + 16)).*;

    // Skip null pointers
    if (pointer_value == 0) {
        return;
    }

    const location_type_str = switch (location.location_type) {
        2 => "Direct",
        3 => "Indirect",
        else => "Other",
    };

    log("gc_root: found pointer 0x{X} at stack location 0x{X} (type={s})", .{ pointer_value, stack_location_addr, location_type_str });

    // Add the actual pointer value to our GC root set
    gc_roots.append(GCRoot{
        .ptr = @ptrFromInt(pointer_value),
        .marked = false,
    }) catch |err| {
        log("gc_root: failed to add root: {}", .{err});
    };
}

fn performGarbageCollection() void {
    log("gc_cycle: starting", .{});
    log("gc_cycle: tracked roots: {}, allocated objects: {}", .{ gc_roots.items.len, allocated_objects.items.len });

    alloc_mutex.lock();
    defer alloc_mutex.unlock();

    // Clear all marks before marking phase
    for (allocated_objects.items) |*obj| {
        obj.marked = false;
    }

    // Mark phase: mark all reachable objects starting from GC roots
    for (gc_roots.items) |*root| {
        root.marked = false;
        markObject(root);
    }

    // Sweep phase: free all unmarked and unprotected objects
    var freed_count: u32 = 0;
    var freed_bytes: u32 = 0;
    var i: usize = 0;
    while (i < allocated_objects.items.len) {
        const obj = &allocated_objects.items[i];

        // Only free if object is not marked
        if (!obj.marked) {
            // Object is not reachable and not protected, free it
            log("gc_sweep: freeing object at 0x{X} (size={})", .{ @intFromPtr(obj.ptr), obj.size });

            // Create a slice from the pointer and size to free it properly
            const bytes = @as([*]u8, @ptrCast(obj.ptr))[0..obj.size];
            allocator.free(bytes);

            freed_count += 1;
            freed_bytes += obj.size;

            // Remove from tracking list by swapping with last element
            _ = allocated_objects.swapRemove(i);
            // Don't increment i since we swapped a new element to this position
        } else {
            i += 1;
        }
    }

    log("gc_cycle: complete, freed {} objects ({} bytes), {} objects remaining", .{ freed_count, freed_bytes, allocated_objects.items.len });
}

fn markObject(root: *GCRoot) void {
    root.marked = true;
    log("gc_mark: root at 0x{X}", .{@intFromPtr(root.ptr)});

    // Find the corresponding allocated object and mark it
    markAllocatedObject(@intFromPtr(root.ptr));

    // TODO: traverse object references here to mark transitively reachable objects
}

fn markAllocatedObject(ptr_addr: usize) void {
    for (allocated_objects.items) |*obj| {
        const obj_addr = @intFromPtr(obj.ptr);
        const obj_end = obj_addr + obj.size;

        // Check if the pointer points into this allocated object
        if (ptr_addr >= obj_addr and ptr_addr < obj_end) {
            if (!obj.marked) {
                obj.marked = true;
                log("gc_mark: allocated object at 0x{X} (size={})", .{ obj_addr, obj.size });

                // TODO: Scan this object for references to other objects
                // For now, we only mark the directly referenced object
            }
            break;
        }
    }
}

export fn gc_safepoint_slow_path() void {
    const frame_addr = @frameAddress();
    const return_addr = @returnAddress();
    log("===GC START===", .{});
    log("gc_safepoint_slow_path: caller_frame_addr: 0x{X}", .{frame_addr});
    // Get the frame address of the calling function
    // Process the stack map at the safepoint to find GC roots
    processStackMapAtSafepoint(return_addr, frame_addr);
    // TODO: Trigger GC based on good heuristics
    performGarbageCollection();
    log("===GC END===", .{});
}

export fn gc_track_roots() void {}

export fn gc_alloc(size: u32) ?*anyopaque {
    // Get the frame address of the calling function
    // Process the stack map at the safepoint to find GC roots
    const bytes = allocator.alloc(u8, size) catch return null;

    // Track the allocated object
    alloc_mutex.lock();
    defer alloc_mutex.unlock();

    allocated_objects.append(AllocatedObject{ .ptr = bytes.ptr, .size = size, .marked = false }) catch |err| {
        log("gc_alloc: failed to track allocation: {}", .{err});
        // Still return the allocation even if tracking fails
    };

    log("gc_alloc: {} bytes at 0x{X}, total objects: {}", .{ size, @intFromPtr(bytes.ptr), allocated_objects.items.len });
    return bytes.ptr;
}
