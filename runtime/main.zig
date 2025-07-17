const std = @import("std");
const types = @import("stackmap_types.zig");

// Import the stackmap and GC types
const StackMapHeader = types.StackMapHeader;
const StkSizeRecord = types.StackSizeRecord; // Note: StackSizeRecord in the types file
const LiveOut = types.LiveOut;
const StackMapRecord = types.StackMapRecord;
const Location = types.Location;
const GCRoot = types.GCRoot;

var gpa = std.heap.GeneralPurposeAllocator(.{}){};
const allocator = gpa.allocator();

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

fn processStackMapAtSafepoint(return_addr: usize) void {
    const stack_map = getStackMap().?; // Unwrap the optional since it panics if null

    // Find the record for this safepoint
    var record_ptr = @as([*]u8, @ptrCast(stack_map)) + @sizeOf(StackMapHeader);

    // Skip function records (each StkSizeRecord) but save function information
    var function_start_addr: u64 = 0;
    for (0..stack_map.num_functions) |_| {
        const func_record = @as(*StkSizeRecord, @ptrCast(@alignCast(record_ptr)));
        // Find which function our return address belongs to
        if (return_addr >= func_record.function_address) {
            function_start_addr = func_record.function_address;
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

    // Examine ALL records to find the statepoint record
    var found_matching_record = false;
    for (0..stack_map.num_records) |_| {
        // Ensure proper alignment for the record structure (8-byte aligned)
        const aligned_addr = std.mem.alignForward(usize, @intFromPtr(record_ptr), 8);
        record_ptr = @as([*]u8, @ptrFromInt(aligned_addr));

        const record = @as(*StackMapRecord, @ptrCast(@alignCast(record_ptr)));

        // Check if this record matches our instruction offset (or is close)
        const offset_match = (record.instruction_offset == instruction_offset) or
            (instruction_offset > record.instruction_offset and instruction_offset < record.instruction_offset + 20);

        const is_statepoint = record.patchpoint_id == 2882400000;
        const is_matching_statepoint = offset_match and is_statepoint;

        if (is_statepoint) {
            if (offset_match) {
                found_matching_record = true;
            }

            // ALWAYS parse statepoint records to see what they contain
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

                if (remaining_locations > 0) {
                    if (remaining_locations % 2 == 0) {
                        const num_relocation_pairs = remaining_locations / 2;

                        for (0..num_relocation_pairs) |_| {
                            // Base pointer location
                            const base_location = @as(*Location, @ptrCast(@alignCast(ptr)));
                            ptr += @sizeOf(Location);

                            // Derived pointer location
                            const derived_location = @as(*Location, @ptrCast(@alignCast(ptr)));
                            ptr += @sizeOf(Location);

                            // Track GC roots from ANY statepoint record for testing
                            if (base_location.location_type == 2 or base_location.location_type == 3) {
                                trackGCRoot(base_location);
                            }

                            if (derived_location.location_type == 2 or derived_location.location_type == 3) {
                                trackGCRoot(derived_location);
                            }
                        }
                    } else {
                        std.debug.panic("error: odd number of remaining locations ({}) - cannot form pairs", .{remaining_locations});
                    }
                }
            }
        }

        var ptr = record_ptr + @sizeOf(StackMapRecord);

        // Skip locations
        ptr += record.num_locations * @sizeOf(Location);

        // Align to 2-byte boundary after locations
        ptr = @as([*]u8, @ptrFromInt(std.mem.alignForward(usize, @intFromPtr(ptr), 2)));

        // Read number of LiveOuts (uint16)
        const num_liveouts = @as(*u16, @ptrCast(@alignCast(ptr))).*;
        ptr += @sizeOf(u16);

        // Skip LiveOut entries (4 bytes each)
        ptr += num_liveouts * @sizeOf(LiveOut);

        // Align to 8-byte boundary for next record
        record_ptr = @as([*]u8, @ptrFromInt(std.mem.alignForward(usize, @intFromPtr(ptr), 8)));

        if (is_matching_statepoint) {
            break; // Found our record, no need to continue
        }
    }
}

fn trackGCRoot(location: *Location) void {
    // Calculate actual address based on location info
    const frame_addr = @frameAddress();

    // Validate location before processing
    if (location.location_size == 0) {
        log("gc_root: skipping zero-size location (type={})", .{location.location_type});
        return;
    }

    const root_addr = switch (location.location_type) {
        2 => blk: { // Direct - value at stack offset
            const offset = @as(isize, location.offset_or_constant);
            const addr = frame_addr + @as(usize, @bitCast(offset));
            log("gc_root: direct access at frame+{} = 0x{X}", .{ offset, addr });
            break :blk addr;
        },
        3 => blk: { // Indirect - read pointer from stack location
            const offset = @as(isize, location.offset_or_constant);
            const ptr_addr = frame_addr + @as(usize, @bitCast(offset));
            log("gc_root: indirect read from frame+{} = 0x{X}", .{ offset, ptr_addr });

            // Safety check: ensure we're reading from a reasonable stack location
            if (ptr_addr < frame_addr - 4096 or ptr_addr > frame_addr + 4096) {
                log("gc_root: skipping unsafe indirect access at 0x{X}", .{ptr_addr});
                return;
            }

            const value = @as(*usize, @ptrFromInt(ptr_addr)).*;
            log("gc_root: indirect value = 0x{X}", .{value});
            break :blk value;
        },
        else => {
            log("gc_root: unsupported location type {}", .{location.location_type});
            return;
        },
    };

    // Skip null pointers
    if (root_addr == 0) {
        log("gc_root: skipping null pointer", .{});
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
    }) catch |err| {
        log("gc_root: failed to add root: {}", .{err});
    };
}

fn performGarbageCollection() void {
    log("gc_cycle: starting", .{});
    log("gc_cycle: tracked roots: {}", .{gc_roots.items.len});

    // Mark phase: mark all reachable objects
    for (gc_roots.items) |*root| {
        markObject(root);
    }

    // Sweep phase would go here...
    log("gc_cycle: complete", .{});

    // Clear roots for next cycle
    gc_roots.clearRetainingCapacity();
}

fn markObject(root: *GCRoot) void {
    if (root.marked) return;
    root.marked = true;
    log("gc_mark: object at 0x{X} (size={})", .{ @intFromPtr(root.ptr), root.size });
    // TODO: traverse object references here
}

export fn gc_safepoint_slow_path() void {
    const return_addr = @returnAddress();
    // Process the stack map at the safepoint to find GC roots
    processStackMapAtSafepoint(return_addr);
    // TODO: Trigger GC based on good heuristics
    performGarbageCollection();
}

export fn gc_alloc(size: u32) ?*anyopaque {
    const bytes = allocator.alloc(u8, size) catch return null;
    log("gc_alloc: {} bytes", .{size});
    return bytes.ptr;
}
