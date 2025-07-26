//! LLVM Stackmap Data Structures and Stack Walking
//!
//! This file contains the data structures that mirror LLVM's stackmap format
//! and provides functionality for walking the stack and parsing stackmaps to
//! find GC roots. LLVM generates stackmaps to track GC roots and other metadata
//! at specific program points (safepoints). The stackmap is emitted as binary
//! data in a special section (__llvm_stackmaps) that can be parsed at runtime.
//!
//! The overall memory layout of an LLVM stackmap section is:
//! +-------------------+
//! | StackMapHeader    | <- Header with counts and version
//! +-------------------+
//! | StkSizeRecord[]   | <- One per function (stack frame sizes)
//! +-------------------+
//! | Constants[]       | <- u64 constants referenced by locations
//! +-------------------+
//! | StackMapRecord[]  | <- One per statepoint/patchpoint
//! +-------------------+
//!
//! Each StackMapRecord is followed by:
//! - Location[] array (variable size based on num_locations)
//! - Padding to 2-byte alignment
//! - u16 num_liveouts
//! - LiveOut[] array
//! - Padding to 8-byte alignment for next record
//!
//! DETAILED STACKMAP RECORD + LOCATIONS LAYOUT:
//! +==============================================+
//! | StackMapRecord (16 bytes)                   |
//! | +------------------------------------------+ |
//! | | patchpoint_id: 2882400000 (statepoint)  | |
//! | | instruction_offset: 0x1234               | |
//! | | reserved: 0                              | |
//! | | num_locations: 8                         | |
//! | +------------------------------------------+ |
//! +==============================================+
//! | Location[0]: Calling Convention (12 bytes)  |
//! | +------------------------------------------+ |
//! | | location_type: 4 (Constant)             | |
//! | | offset_or_constant: 64 (C calling conv) | |
//! | +------------------------------------------+ |
//! +==============================================+
//! | Location[1]: Flags (12 bytes)               |
//! | +------------------------------------------+ |
//! | | location_type: 4 (Constant)             | |
//! | | offset_or_constant: 0 (no flags)        | |
//! | +------------------------------------------+ |
//! +==============================================+
//! | Location[2]: Deopt Args Count (12 bytes)    |
//! | +------------------------------------------+ |
//! | | location_type: 4 (Constant)             | |
//! | | offset_or_constant: 0 (no deopt args)   | |
//! | +------------------------------------------+ |
//! +==============================================+
//! | Location[3]: GC Base Pointer (12 bytes)     |
//! | +------------------------------------------+ |
//! | | location_type: 2 (Direct)               | | ─┐
//! | | location_size: 8 (pointer size)         | |  │
//! | | offset_or_constant: -16 (stack offset)  | |  │ GC ROOT PAIR
//! | +------------------------------------------+ |  │ (base + derived)
//! +==============================================+  │
//! | Location[4]: GC Derived Pointer (12 bytes)  |  │
//! | +------------------------------------------+ | ─┘
//! | | location_type: 3 (Indirect)             | |
//! | | location_size: 8 (pointer size)         | |
//! | | offset_or_constant: -8 (stack offset)   | |
//! | +------------------------------------------+ |
//! +==============================================+
//! | Location[5]: Another GC Base (12 bytes)     |
//! | +------------------------------------------+ | ─┐
//! | | location_type: 1 (Register)             | |  │
//! | | location_size: 8 (pointer size)         | |  │ ANOTHER GC PAIR
//! | | dwarf_reg_num: 5 (register %rbp)        | |  │ (register-based)
//! | +------------------------------------------+ |  │
//! +==============================================+  │
//! | Location[6]: Another GC Derived (12 bytes)   |  │
//! | +------------------------------------------+ | ─┘
//! | | location_type: 5 (ConstantIndex)        | |
//! | | location_size: 8 (pointer size)         | |
//! | | offset_or_constant: 0 (constants[0])    | |
//! | +------------------------------------------+ |
//! +==============================================+
//! | Location[7]: GC Object Field (12 bytes)     |
//! | +------------------------------------------+ | ─┐
//! | | location_type: 2 (Direct)               | |  │ SINGLE GC ROOT
//! | | location_size: 8 (pointer size)         | |  │ (field pointer)
//! | | offset_or_constant: -24 (stack offset)  | |  │
//! | +------------------------------------------+ | ─┘
//! +==============================================+
//! | Padding to 2-byte alignment                 |
//! +==============================================+
//! | num_liveouts: 3 (u16)                       |
//! +==============================================+
//! | LiveOut[0-2] entries (4 bytes each)         |
//! +==============================================+
//! | Padding to 8-byte alignment for next record |
//! +==============================================+
//!
//! LOCATION TYPE → GC ROOT MAPPING:
//!
//! Type 1 (Register): Value in CPU register
//! ┌─────────────────┐    ┌─────────────────┐
//! │ dwarf_reg_num:5 │ ──→│ Register %rbp   │ ──→ [GC Object*]
//! │ (location info) │    │ (runtime)       │
//! └─────────────────┘    └─────────────────┘
//!
//! Type 2 (Direct): Value at stack offset
//! ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
//! │ offset: -16     │ ──→│ Frame Ptr - 16  │ ──→│ [GC Object*]    │
//! │ (location info) │    │ (stack address) │    │ (actual object) │
//! └─────────────────┘    └─────────────────┘    └─────────────────┘
//!
//! Type 3 (Indirect): Pointer to value at stack offset
//! ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
//! │ offset: -8      │ ──→│ Frame Ptr - 8   │ ──→│ [Pointer*]      │ ──→│ [GC Object*]    │
//! │ (location info) │    │ (stack address) │    │ (intermediate)  │    │ (actual object) │
//! └─────────────────┘    └─────────────────┘    └─────────────────┘    └─────────────────┘
//!
//! Type 4 (Constant): Compile-time constant value
//! ┌─────────────────┐    ┌─────────────────┐
//! │ constant: 0x0   │ ──→│ NULL pointer    │ (typically ignored for GC)
//! │ (location info) │    │ (literal value) │
//! └─────────────────┘    └─────────────────┘
//!
//! Type 5 (ConstantIndex): Index into constants table
//! ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
//! │ index: 0        │ ──→│ Constants[0]    │ ──→│ [GC Object*]    │
//! │ (location info) │    │ (u64 value)     │    │ (actual object) │
//! └─────────────────┘    └─────────────────┘    └─────────────────┘
//!
//! CONSTANTS SECTION DETAILS:
//! The constants section contains u64 values referenced by Location entries
//! with location_type 5 (ConstantIndex). These constants typically hold:
//!
//! 1. LARGE IMMEDIATE VALUES: Values that don't fit in the 32-bit offset_or_constant field
//!    Example: Constants[0] = 0x123456789ABCDEF0 (64-bit literal)
//!
//! 2. OBJECT ADDRESSES: Pre-computed addresses of static objects or metadata
//!    Example: Constants[1] = 0x00007FF8A0001000 (static object address)
//!
//! 3. GC METADATA: Type descriptors, object headers, or GC-specific information
//!    Example: Constants[2] = 0x0000000000001234 (type ID or metadata pointer)
//!
//! 4. FUNCTION POINTERS: Addresses of functions for indirect calls at statepoints
//!    Example: Constants[3] = 0x00007FF8A0005678 (function entry point)
//!
//! 5. STRING/DATA ADDRESSES: Pointers to constant strings or data sections
//!    Example: Constants[4] = 0x00007FF8A000ABCD (string literal address)
//!
//! Usage in Location entries:
//! When location_type = 5 (ConstantIndex), the offset_or_constant field
//! contains an index into this constants array:
//!   offset_or_constant = 2  →  value = Constants[2]
//!
//! Memory Layout of Constants Section:
//! +================================+
//! | Constants[0]: 0x123...DEF0 (8) | ← Large immediate value
//! +================================+
//! | Constants[1]: 0x7FF...1000 (8) | ← Static object address
//! +================================+
//! | Constants[2]: 0x000...1234 (8) | ← Type metadata
//! +================================+
//! | Constants[3]: 0x7FF...5678 (8) | ← Function pointer
//! +================================+
//! | Constants[4]: 0x7FF...ABCD (8) | ← String address
//! +================================+
//!
//! GC ROOT EXTRACTION PROCESS:
//! 1. Parse statepoint record (patchpoint_id == 2882400000)
//! 2. Skip first 3 locations (calling convention, flags, deopt count)
//! 3. Skip deopt argument locations (if any)
//! 4. Process remaining locations in pairs (base + derived) for GC relocations
//! 5. Extract runtime addresses using location type-specific logic:
//!    - For ConstantIndex locations: dereference Constants[offset_or_constant]
//! 6. Add extracted pointers to GC root set for marking phase

const std = @import("std");

/// Header of the LLVM stackmap section, always appears first.
/// This header provides metadata about the entire stackmap section
/// and counts for each subsequent data type.
///
/// Memory layout: 16 bytes total
/// +0:  version (u8)        - Stackmap format version (typically 3)
/// +1:  reserved1 (u8)      - Reserved, must be 0
/// +2:  reserved2 (u16)     - Reserved, must be 0
/// +4:  num_functions (u32) - Count of StkSizeRecord entries
/// +8:  num_constants (u32) - Count of u64 constant values
/// +12: num_records (u32)   - Count of StackMapRecord entries
pub const StackMapHeader = extern struct {
    /// Stackmap format version. Current version is typically 3.
    /// Determines the layout and interpretation of subsequent data.
    version: u8,

    /// Reserved field, always 0. Provides alignment and future extensibility.
    reserved1: u8,

    /// Reserved field, always 0. Provides alignment and future extensibility.
    reserved2: u16,

    /// Number of function entries (StkSizeRecord) that follow the header.
    /// Each function that contains statepoints gets an entry here.
    num_functions: u32,

    /// Number of 64-bit constants that follow the function records.
    /// These are literal values referenced by Location entries.
    num_constants: u32,

    /// Number of stackmap records (StackMapRecord) in this section.
    /// Each statepoint/patchpoint generates one record.
    num_records: u32,
};

/// Per-function metadata record describing stack frame information.
/// Appears immediately after StackMapHeader, one entry per function.
/// These records provide stack size information for functions containing
/// statepoints, which is useful for stack walking and frame analysis.
///
/// Memory layout: 24 bytes total
/// +0:  function_address (u64) - Start address of the function
/// +8:  stack_size (u64)       - Size of stack frame in bytes
/// +16: record_count (u64)     - Number of StackMapRecord entries for this function
pub const StackSizeRecord = extern struct {
    /// The runtime address where this function begins.
    /// Used to correlate instruction offsets in StackMapRecord entries
    /// back to this specific function.
    function_address: u64,

    /// Size of this function's stack frame in bytes.
    /// Useful for stack walking and determining frame boundaries.
    stack_size: u64,

    /// Number of StackMapRecord entries that belong to this function.
    /// Helps with parsing and validation of the stackmap data.
    record_count: u64,
};

/// Describes a register that contains a live value at a safepoint.
/// LiveOut entries appear after each StackMapRecord and describe
/// registers that are live (contain meaningful values) at the safepoint.
/// This is primarily used for debugging and register allocation info.
///
/// Memory layout: 4 bytes total
/// +0: reg_num (u16)        - DWARF register number
/// +2: reserved (u8)        - Reserved, must be 0
/// +3: size_in_bytes (u8)   - Size of value in register
pub const LiveOut = extern struct {
    /// DWARF register number identifying which register contains the live value.
    /// Uses the DWARF register numbering convention for the target architecture.
    reg_num: u16,

    /// Reserved field for alignment, always 0.
    reserved: u8,

    /// Size of the live value in the register, in bytes.
    /// Typically 4 for 32-bit values, 8 for 64-bit values.
    size_in_bytes: u8,
};

/// Core record describing a single statepoint or patchpoint.
/// This is the primary data structure for each safepoint where GC
/// information is recorded. For statepoints (GC safepoints), the
/// Location array that follows contains GC root information.
///
/// Memory layout: 16 bytes total
/// +0:  patchpoint_id (u64)     - Unique ID (2882400000 for statepoints)
/// +8:  instruction_offset (u32) - Offset from function start
/// +12: reserved (u16)          - Reserved, must be 0
/// +14: num_locations (u16)     - Count of Location entries that follow
pub const StackMapRecord = extern struct {
    /// Unique identifier for this patchpoint/statepoint.
    /// - 2882400000 (0xABCDEF00) indicates a statepoint (GC safepoint)
    /// - Other values indicate different types of patchpoints
    patchpoint_id: u64,

    /// Byte offset from the start of the function to this statepoint instruction.
    /// Combined with function_address from StkSizeRecord, gives absolute address.
    instruction_offset: u32,

    /// Reserved field for alignment, always 0.
    reserved: u16,

    /// Number of Location entries that immediately follow this record.
    /// For statepoints, includes calling convention, flags, deopt args, and GC relocations.
    num_locations: u16,
};

/// Describes the location of a value at a safepoint.
/// Location entries immediately follow each StackMapRecord and encode
/// where values (especially GC pointers) are stored at the safepoint.
/// For statepoints, the locations encode GC relocation information.
///
/// Memory layout: 12 bytes total
/// +0:  location_type (u8)        - Type of location (register, stack, etc.)
/// +1:  reserved_1 (u8)           - Reserved, must be 0
/// +2:  location_size (u16)       - Size of value in bytes
/// +4:  dwarf_reg_num (u16)       - DWARF register number (if applicable)
/// +6:  reserved_2 (u16)          - Reserved, must be 0
/// +8:  offset_or_constant (i32)  - Offset, constant, or register value
pub const Location = extern struct {
    /// Type of location where the value is stored:
    /// - 1: Register - value is in a CPU register
    /// - 2: Direct - value is at stack offset (frame pointer + offset)
    /// - 3: Indirect - value is pointed to by (frame pointer + offset)
    /// - 4: Constant - value is a compile-time constant
    /// - 5: ConstantIndex - value is a constant from the constants table
    location_type: u8,

    /// Reserved field for alignment, always 0.
    reserved_1: u8,

    /// Size of the value in bytes. For pointers, typically 8 on 64-bit systems.
    location_size: u16,

    /// DWARF register number when location_type is Register.
    /// Ignored for other location types but may contain padding data.
    dwarf_reg_num: u16,

    /// Reserved field for alignment, always 0.
    reserved_2: u16,

    /// Interpretation depends on location_type:
    /// - Register: register value or additional register info
    /// - Direct: signed offset from frame pointer to value
    /// - Indirect: signed offset from frame pointer to pointer that points to value
    /// - Constant: the actual constant value (sign-extended if needed)
    /// - ConstantIndex: index into the constants table
    offset_or_constant: i32,
};

// === STACK WALKING AND PARSING IMPLEMENTATION ===

const debug = @import("debug.zig");

// Import libunwind for stack walking
const c = @cImport({
    @cInclude("libunwind.h");
    @cInclude("mach-o/getsect.h");
    @cInclude("mach-o/dyld.h");
});

// Structure to represent a stack frame
const StackFrame = struct {
    instruction_pointer: usize,
    frame_pointer: usize,
};

// LLVM places the stack map in a special section
// We access it via linker symbols that LLVM generates
// Define as optional pointer to handle when symbol doesn't exist
var llvm_stackmaps_ptr: ?*StackMapHeader = null;
var llvm_stackmaps_size: usize = 0;
var symbol_lookup_attempted: bool = false;

// Helper function to extract a pointer from a location
fn extractPointerFromLocation(allocator: std.mem.Allocator, location: *Location, frame_addr: usize) ?*anyopaque {
    // Only process Direct and Indirect locations that contain pointers
    if (location.location_type != 2 and location.location_type != 3) {
        return null;
    }

    // Skip if location size is not pointer-sized
    if (location.location_size != @sizeOf(*anyopaque)) {
        return null;
    }

    const stack_location_addr = blk: {
        const offset = @as(isize, location.offset_or_constant);
        break :blk frame_addr + @as(usize, @bitCast(offset));
    };

    // Basic bounds checking - ensure the address looks reasonable
    if (stack_location_addr == 0 or stack_location_addr < 0x1000) {
        debug.log(allocator, "gc_stackmap", "invalid stack location 0x{X}", .{stack_location_addr});
        return null;
    }

    const pointer_value_ptr = @as(*usize, @ptrFromInt(stack_location_addr));
    const pointer_value = pointer_value_ptr.*;

    // Skip null pointers
    if (pointer_value == 0) {
        return null;
    }

    const location_type_str = switch (location.location_type) {
        2 => "Direct",
        3 => "Indirect",
        else => "Other",
    };

    debug.log(allocator, "gc_stackmap", "found pointer 0x{X} at stack location 0x{X} (type={s})", .{ pointer_value, stack_location_addr, location_type_str });

    return @ptrFromInt(pointer_value);
}

// Helper function to dump raw hex data from stackmap section
fn dumpStackMapHex(allocator: std.mem.Allocator, data: [*]const u8, size: usize) void {
    debug.log(allocator, "stackmap_hex", "Stackmap section size: {} bytes", .{size});
    debug.log(allocator, "stackmap_hex", "Raw hex dump (first 256 bytes):", .{});

    const max_dump = @min(size, 256); // Limit output to avoid spam
    var i: usize = 0;
    while (i < max_dump) {
        const bytes_this_line = @min(16, max_dump - i);

        // Just dump the raw bytes for each line
        var j: usize = 0;
        while (j < bytes_this_line) {
            debug.log(allocator, "stackmap_hex", "byte[{}]: 0x{X}", .{ i + j, data[i + j] });
            j += 1;
        }

        i += 16;
    }
}

// Try to locate the LLVM stack map section at runtime
fn tryFindStackMapSymbol() ?*StackMapHeader {
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

    // Store the size for hex dumping
    llvm_stackmaps_size = @intCast(size);

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

// Walk the stack using libunwind and collect all frames
// This provides more accurate stack traversal compared to manual frame walking,
// handling various calling conventions, optimized code, and exception unwinding correctly
fn walkStack(allocator: std.mem.Allocator) !std.ArrayList(StackFrame) {
    var frames = std.ArrayList(StackFrame).init(allocator);
    errdefer frames.deinit();

    var cursor: c.unw_cursor_t = undefined;
    var context: c.unw_context_t = undefined;

    // Get the current machine context (registers, stack pointer, etc.)
    const get_context_result = c.unw_getcontext(&context);
    if (get_context_result != 0) {
        debug.log(allocator, "libunwind", "failed to get context: {}", .{get_context_result});
        return error.UnwindGetContextFailed;
    }

    // Initialize cursor for local unwinding (same process)
    const init_local_result = c.unw_init_local(&cursor, &context);
    if (init_local_result != 0) {
        debug.log(allocator, "libunwind", "failed to init local: {}", .{init_local_result});
        return error.UnwindInitLocalFailed;
    }

    var frame_count: u32 = 0;
    const max_frames: u32 = 10000; // Prevent infinite loops in case of stack corruption

    // Walk the stack frame by frame
    while (frame_count < max_frames) {
        var ip: c.unw_word_t = 0;
        var sp: c.unw_word_t = 0;

        // Get instruction pointer (where we are in the code)
        const get_reg_ip_result = c.unw_get_reg(&cursor, c.UNW_REG_IP, &ip);
        if (get_reg_ip_result != 0) {
            debug.log(allocator, "libunwind", "failed to get IP: {}", .{get_reg_ip_result});
            break;
        }

        // Get stack pointer (current frame's stack location)
        const get_reg_sp_result = c.unw_get_reg(&cursor, c.UNW_REG_SP, &sp);
        if (get_reg_sp_result != 0) {
            debug.log(allocator, "libunwind", "failed to get SP: {}", .{get_reg_sp_result});
            break;
        }

        // Skip frames that might be in system libraries or have invalid addresses
        if (ip == 0 or sp == 0) {
            debug.log(allocator, "gc_stackwalk", "skipping frame with null IP or SP", .{});
            const step_result = c.unw_step(&cursor);
            if (step_result <= 0) break;
            frame_count += 1;
            continue;
        }

        // Add frame to our collection
        try frames.append(StackFrame{
            .instruction_pointer = @intCast(ip),
            .frame_pointer = @intCast(sp),
        });

        debug.log(allocator, "gc_stackwalk", "frame {}: IP=0x{X}, SP=0x{X}", .{ frame_count, ip, sp });

        // Move to the next frame up the stack
        const step_result = c.unw_step(&cursor);
        if (step_result <= 0) {
            if (step_result < 0) {
                debug.log(allocator, "libunwind", "step failed: {}", .{step_result});
            }
            break; // End of stack reached or unwind error
        }

        frame_count += 1;
    }

    if (frame_count >= max_frames) {
        debug.log(allocator, "gc_stackwalk", "reached maximum frame limit, possible stack corruption", .{});
    }

    debug.log(allocator, "gc_stackwalk", "collected {} frames", .{frames.items.len});
    return frames;
}

fn processStackMapAtSafepoint(allocator: std.mem.Allocator, return_addr: usize, caller_frame_addr: usize, pointers: *std.ArrayList(*anyopaque)) void {
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
        debug.log(allocator, "gc_stackmap", "no matching function found for return address 0x{X}", .{return_addr});
        return;
    }

    // Calculate how many records we need to skip to get to our function's records
    record_ptr = @as([*]u8, @ptrCast(stack_map)) + @sizeOf(StackMapHeader);
    for (0..target_function_index.?) |_| {
        const func_record = @as(*StackSizeRecord, @ptrCast(@alignCast(record_ptr)));
        records_to_skip += func_record.record_count;
        record_ptr += @sizeOf(StackSizeRecord);
    }

    // Skip remaining functions to get to the records section
    for (target_function_index.?..stack_map.num_functions) |_| {
        record_ptr += @sizeOf(StackSizeRecord);
    }

    // Skip constants
    record_ptr += stack_map.num_constants * @sizeOf(u64);

    // Calculate instruction offset from return address
    const instruction_offset = if (return_addr > function_start_addr)
        @as(u32, @intCast(return_addr - function_start_addr))
    else
        0;

    debug.log(allocator, "gc_stackmap", "looking for instruction_offset: {} in function {} (skipping {} records, searching {} records)", .{ instruction_offset, target_function_index.?, records_to_skip, target_function_record_count });

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
            debug.log(allocator, "gc_stackmap", "found matching statepoint record at offset {}", .{instruction_offset});
            processStateMapRecord(allocator, record, caller_frame_addr, pointers);
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

fn processStateMapRecord(allocator: std.mem.Allocator, record: *StackMapRecord, caller_frame_addr: usize, pointers: *std.ArrayList(*anyopaque)) void {
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
    debug.log(allocator, "gc_stackmap", "remaining_locations: {}", .{remaining_locations});

    if (remaining_locations > 0 and remaining_locations % 2 == 0) {
        const num_relocation_pairs = remaining_locations / 2;
        debug.log(allocator, "gc_stackmap", "num_relocation_pairs: {}", .{num_relocation_pairs});

        for (0..num_relocation_pairs) |_| {
            // Base pointer location
            const base_location = @as(*Location, @ptrCast(@alignCast(ptr)));
            ptr += @sizeOf(Location);

            // Derived pointer location
            const derived_location = @as(*Location, @ptrCast(@alignCast(ptr)));
            ptr += @sizeOf(Location);

            // Extract and collect GC root pointers
            if (extractPointerFromLocation(allocator, base_location, caller_frame_addr)) |base_ptr| {
                pointers.append(base_ptr) catch |err| {
                    debug.log(allocator, "gc_stackmap", "failed to add base pointer: {}", .{err});
                };
            }
            if (extractPointerFromLocation(allocator, derived_location, caller_frame_addr)) |derived_ptr| {
                pointers.append(derived_ptr) catch |err| {
                    debug.log(allocator, "gc_stackmap", "failed to add derived pointer: {}", .{err});
                };
            }
        }
    }
}

/// Walk the entire stack and return a list of GC root pointers found.
/// This is the main public interface for stack walking and GC root discovery.
///
/// Returns an ArrayList of pointers that should be tracked as GC roots.
/// The caller is responsible for deinitializing the returned ArrayList.
pub fn walkStackAndProcessRoots(allocator: std.mem.Allocator) !std.ArrayList(*anyopaque) {
    var pointers = std.ArrayList(*anyopaque).init(allocator);
    errdefer pointers.deinit();

    const frames = walkStack(allocator) catch |err| {
        debug.log(allocator, "gc_stackwalk", "failed to walk stack: {}", .{err});
        return err;
    };
    defer frames.deinit();

    debug.log(allocator, "gc_stackwalk", "processing {} stack frames", .{frames.items.len});

    // Process each frame
    for (frames.items) |frame| {
        processStackMapAtSafepoint(allocator, frame.instruction_pointer, frame.frame_pointer, &pointers);
    }

    return pointers;
}
