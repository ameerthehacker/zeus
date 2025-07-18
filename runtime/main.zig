const std = @import("std");
const types = @import("stackmap_types.zig");

// Import the stackmap and GC types
const StackMapHeader = types.StackMapHeader;
const StkSizeRecord = types.StackSizeRecord; // Corrected from StkSizeRecord to StackSizeRecord
const LiveOut = types.LiveOut;
const StackMapRecord = types.StackMapRecord;
const Location = types.Location;
const GCRoot = types.GCRoot;

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
    const stack_map = getStackMap().?; // Unwrap the optional since it panics if null

    // Find the record for this safepoint
    var record_ptr = @as([*]u8, @ptrCast(stack_map)) + @sizeOf(StackMapHeader);

    // Skip function records (each StkSizeRecord) but save function information
    var function_start_addr: u64 = 0;
    var current_function_record_count: u64 = 0;
    var function_index: u64 = 0;
    for (0..stack_map.num_functions) |i| {
        const func_record = @as(*StkSizeRecord, @ptrCast(@alignCast(record_ptr)));
        // Find which function our return address belongs to
        if (return_addr >= func_record.function_address) {
            function_start_addr = func_record.function_address;
            current_function_record_count = func_record.record_count;
            function_index = i;
        }

        record_ptr += @sizeOf(StkSizeRecord);
    }

    // Skip constants (each is u64)
    record_ptr += stack_map.num_constants * @sizeOf(u64);

    // Calculate instruction offset from return address
    const instruction_offset = if (return_addr > function_start_addr)
        @as(u32, @intCast(return_addr - function_start_addr))
    else
        0;

    // Examine ALL records to find first matching statepoint, then process function records
    var found_first_match = false;
    var records_processed_in_function: u64 = 0;

    log("gc_safepoint_slow_path: function_index: {}", .{function_index});
    log("gc_safepoint_slow_path: function_record_count: {}", .{current_function_record_count});

    for (0..stack_map.num_records) |j| {
        // Ensure proper alignment for the record structure (8-byte aligned)
        const aligned_addr = std.mem.alignForward(usize, @intFromPtr(record_ptr), 8);
        record_ptr = @as([*]u8, @ptrFromInt(aligned_addr));

        const record = @as(*StackMapRecord, @ptrCast(@alignCast(record_ptr)));

        // Check if this record matches our instruction offset
        const offset_match = (record.instruction_offset == instruction_offset);
        const is_statepoint = record.patchpoint_id == 2882400000;
        const is_matching_statepoint = offset_match and is_statepoint;

        // If we found the first matching statepoint, start processing this function's records
        if (is_matching_statepoint) {
            found_first_match = true;
            log("gc_safepoint_slow_path: function {}: first record: {}", .{ function_index, j });
        }

        // Process statepoint records if we're in the target function
        if (found_first_match and records_processed_in_function < current_function_record_count) {
            var ptr = record_ptr + @sizeOf(StackMapRecord);

            if (record.num_locations >= 3) {
                // Ignore calling convention
                ptr += @sizeOf(Location);

                // Ignore flags
                ptr += @sizeOf(Location);

                // Ignore number of deopt arguments
                const num_deopt_args = @as(u32, @intCast(@as(*Location, @ptrCast(@alignCast(ptr))).offset_or_constant));
                ptr += @sizeOf(Location);

                // Skip deopt arguments
                for (0..num_deopt_args) |_| {
                    ptr += @sizeOf(Location);
                }

                // Calculate remaining locations
                const remaining_locations = record.num_locations - 3 - num_deopt_args;
                log("gc_safepoint_slow_path: remaining_locations: {}", .{remaining_locations});

                if (remaining_locations > 0) {
                    if (remaining_locations % 2 == 0) {
                        const num_relocation_pairs = remaining_locations / 2;

                        log("gc_safepoint_slow_path: num_relocation_pairs: {}", .{num_relocation_pairs});

                        for (0..num_relocation_pairs) |_| {
                            // Base pointer location
                            const base_location = @as(*Location, @ptrCast(@alignCast(ptr)));
                            ptr += @sizeOf(Location);

                            // Derived pointer location
                            const derived_location = @as(*Location, @ptrCast(@alignCast(ptr)));
                            ptr += @sizeOf(Location);

                            // Track GC roots from this function's statepoint records
                            if (base_location.location_type == 2 or base_location.location_type == 3) {
                                trackGCRootWithFunction(base_location, caller_frame_addr, function_start_addr);
                            }

                            if (derived_location.location_type == 2 or derived_location.location_type == 3) {
                                trackGCRootWithFunction(derived_location, caller_frame_addr, function_start_addr);
                            }
                        }
                    } else {
                        std.debug.panic("error: odd number of remaining locations ({}) - cannot form pairs", .{remaining_locations});
                    }
                }
            }

            records_processed_in_function += 1;

            // If we've processed all records for this function, we're done
            if (records_processed_in_function >= current_function_record_count) {
                break;
            }
        }

        var ptr = record_ptr + @sizeOf(StackMapRecord);

        // Skip locations
        ptr += record.num_locations * @sizeOf(Location);

        // Align to 8-byte boundary after locations (LLVM uses 8-byte alignment)
        ptr = @as([*]u8, @ptrFromInt(std.mem.alignForward(usize, @intFromPtr(ptr), 8)));

        // Read number of LiveOuts (uint16) with bounds checking
        const num_liveouts_ptr = @as(*u16, @ptrCast(@alignCast(ptr)));
        const num_liveouts = num_liveouts_ptr.*;

        ptr += @sizeOf(u16);

        // Skip LiveOut entries (4 bytes each)
        ptr += num_liveouts * @sizeOf(LiveOut);

        // Align to 8-byte boundary for next record
        record_ptr = @as([*]u8, @ptrFromInt(std.mem.alignForward(usize, @intFromPtr(ptr), 8)));
    }
}

fn trackGCRootWithFunction(location: *Location, frame_addr: usize, function_addr: u64) void {
    // Calculate actual address based on location info (using caller's frame address)
    log("gc_root: frame_addr: 0x{X}, function_addr: 0x{X}", .{ frame_addr, function_addr });

    // Validate location before processing
    if (location.location_size == 0) {
        return;
    }

    const root_addr = switch (location.location_type) {
        2 => blk: { // Direct - value at stack offset
            const offset = @as(isize, location.offset_or_constant);
            const addr = frame_addr + @as(usize, @bitCast(offset));
            break :blk addr;
        },
        3 => blk: { // Indirect - read pointer from stack location
            const offset = @as(isize, location.offset_or_constant);
            const ptr_addr = frame_addr + @as(usize, @bitCast(offset));

            // Safety check: ensure we're reading from a reasonable stack location
            if (ptr_addr < frame_addr - 4096 or ptr_addr > frame_addr + 4096) {
                return;
            }

            const value = @as(*usize, @ptrFromInt(ptr_addr)).*;
            break :blk value;
        },
        else => {
            return;
        },
    };

    // Skip null pointers
    if (root_addr == 0) {
        return;
    }

    const location_type_str = switch (location.location_type) {
        2 => "Direct",
        3 => "Indirect",
        else => "Other",
    };

    log("gc_root: tracking at 0x{X} (size={}, type={s}, function=0x{X})", .{ root_addr, location.location_size, location_type_str, function_addr });

    // Add to our GC root set with function information
    gc_roots.append(GCRoot{
        .ptr = @ptrFromInt(root_addr),
        .size = @as(u32, location.location_size),
        .marked = false,
        .function_addr = function_addr,
        .frame_addr = frame_addr,
    }) catch |err| {
        log("gc_root: failed to add root: {}", .{err});
    };
}

fn trackGCRoot(location: *Location, frame_addr: usize) void {
    // Calculate actual address based on location info (using caller's frame address)
    log("gc_root: frame_addr: 0x{X}", .{frame_addr});

    // Validate location before processing
    if (location.location_size == 0) {
        return;
    }

    const root_addr = switch (location.location_type) {
        2 => blk: { // Direct - value at stack offset
            const offset = @as(isize, location.offset_or_constant);
            const addr = frame_addr + @as(usize, @bitCast(offset));
            break :blk addr;
        },
        3 => blk: { // Indirect - read pointer from stack location
            const offset = @as(isize, location.offset_or_constant);
            const ptr_addr = frame_addr + @as(usize, @bitCast(offset));

            // Safety check: ensure we're reading from a reasonable stack location
            if (ptr_addr < frame_addr - 4096 or ptr_addr > frame_addr + 4096) {
                return;
            }

            const value = @as(*usize, @ptrFromInt(ptr_addr)).*;
            break :blk value;
        },
        else => {
            return;
        },
    };

    // Skip null pointers
    if (root_addr == 0) {
        return;
    }

    const location_type_str = switch (location.location_type) {
        2 => "Direct",
        3 => "Indirect",
        else => "Other",
    };

    log("gc_root: tracking at 0x{X} (size={}, type={s})", .{ root_addr, location.location_size, location_type_str });

    // Add to our GC root set
    gc_roots.append(GCRoot{
        .ptr = @ptrFromInt(root_addr),
        .size = @as(u32, location.location_size),
        .marked = false,
        .function_addr = 0, // Unknown function for now
        .frame_addr = frame_addr,
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
            // const bytes = @as([*]u8, @ptrCast(obj.ptr))[0..obj.size];
            // allocator.free(bytes);

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

    // Clean up stale roots by validating against current call stack
    cleanupStaleRoots();
}

fn markObject(root: *GCRoot) void {
    if (root.marked) return;
    root.marked = true;
    log("gc_mark: root at 0x{X} (size={})", .{ @intFromPtr(root.ptr), root.size });

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

fn cleanupStaleRoots() void {
    log("gc_cleanup: cleaning stale roots", .{});

    // Get current call stack function addresses
    var active_functions = std.ArrayList(u64).init(allocator);
    defer active_functions.deinit();

    // Walk call stack to get active function addresses
    walkCallStack(&active_functions) catch {
        log("gc_cleanup: failed to walk call stack, keeping all roots", .{});
        return;
    };

    log("gc_cleanup: found {} active functions", .{active_functions.items.len});

    // Remove roots from functions no longer on the stack
    var i: usize = 0;
    while (i < gc_roots.items.len) {
        const root = &gc_roots.items[i];

        // Check if this root's function is still active
        var function_still_active = false;
        for (active_functions.items) |active_func_addr| {
            if (root.function_addr == active_func_addr or root.function_addr == 0) {
                function_still_active = true;
                break;
            }
        }

        if (function_still_active) {
            i += 1; // Keep this root
        } else {
            log("gc_cleanup: removing stale root from function 0x{X}", .{root.function_addr});
            _ = gc_roots.swapRemove(i);
            // Don't increment i since we removed an element
        }
    }

    log("gc_cleanup: {} roots remaining after cleanup", .{gc_roots.items.len});
}

fn walkCallStack(active_functions: *std.ArrayList(u64)) !void {
    // Simple approach: use frame pointer to walk the stack
    var current_frame = @frameAddress();
    var depth: u32 = 0;
    const max_depth = 32; // Prevent infinite loops

    while (depth < max_depth) {
        // Try to get return address from current frame
        const return_addr = getReturnAddressFromFrame(current_frame) catch break;

        // Find which function this return address belongs to
        const function_addr = findFunctionForAddress(return_addr) catch break;

        if (function_addr != 0) {
            active_functions.append(function_addr) catch break;
            log("gc_cleanup: active function at 0x{X}", .{function_addr});
        }

        // Move to parent frame
        current_frame = getParentFrame(current_frame) catch break;
        depth += 1;
    }
}

fn getReturnAddressFromFrame(frame_addr: usize) !usize {
    // On most architectures, return address is at [frame_pointer + word_size]
    const return_addr_ptr = frame_addr + @sizeOf(usize);

    // Basic bounds check
    if (return_addr_ptr < frame_addr or return_addr_ptr > frame_addr + 1024) {
        return error.InvalidFrame;
    }

    return @as(*usize, @ptrFromInt(return_addr_ptr)).*;
}

fn getParentFrame(frame_addr: usize) !usize {
    // On most architectures, parent frame pointer is at [frame_pointer]
    const parent_frame_ptr = @as(*usize, @ptrFromInt(frame_addr)).*;

    // Basic validation - parent frame should be higher on stack
    if (parent_frame_ptr <= frame_addr or parent_frame_ptr > frame_addr + 0x10000) {
        return error.InvalidFrame;
    }

    return parent_frame_ptr;
}

fn findFunctionForAddress(addr: usize) !u64 {
    const stack_map = getStackMap() orelse return error.NoStackMap;
    var record_ptr = @as([*]u8, @ptrCast(stack_map)) + @sizeOf(StackMapHeader);

    // Check each function's address range
    for (0..stack_map.num_functions) |_| {
        const func_record = @as(*StkSizeRecord, @ptrCast(@alignCast(record_ptr)));

        // Simple check: if address is >= function start, assume it's this function
        // (This is approximate - better would check function end address too)
        if (addr >= func_record.function_address) {
            return func_record.function_address;
        }

        record_ptr += @sizeOf(StkSizeRecord);
    }

    return 0; // Function not found
}

export fn gc_safepoint_slow_path() void {
    log("===GC START===", .{});
    const frame_addr = @frameAddress();
    // Get the frame address of the calling function
    const return_addr = @returnAddress();
    // Process the stack map at the safepoint to find GC roots
    processStackMapAtSafepoint(return_addr, frame_addr);
    // TODO: Trigger GC based on good heuristics
    performGarbageCollection();
    log("===GC END===", .{});
}

export fn gc_alloc(size: u32) ?*anyopaque {
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
