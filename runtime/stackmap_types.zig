//! LLVM Stackmap Data Structures
//!
//! This file contains the data structures that mirror LLVM's stackmap format.
//! LLVM generates stackmaps to track GC roots and other metadata at specific
//! program points (safepoints). The stackmap is emitted as binary data in a
//! special section (__llvm_stackmaps) that can be parsed at runtime.
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

/// Runtime representation of a GC root for the garbage collector.
/// This is NOT part of the LLVM stackmap format, but rather our
/// runtime data structure for tracking GC roots extracted from
/// the stackmap Location entries.
///
/// Memory layout: 16 bytes total (on 64-bit)
/// +0:  ptr (*anyopaque)  - Pointer to the GC-managed object
/// +8:  size (u32)        - Size of the object in bytes  
/// +12: marked (bool)     - GC mark bit for mark-and-sweep
/// +13: [padding]         - Alignment padding
pub const GCRoot = struct {
    /// Pointer to the start of the GC-managed object.
    /// This address is extracted from stackmap Location entries
    /// and represents a live GC pointer at a safepoint.
    ptr: *anyopaque,
    
    /// Size of the GC object in bytes.
    /// Extracted from the location_size field of the corresponding Location.
    size: u32,
    
    /// Mark bit used during garbage collection mark phase.
    /// Set to true when the object is determined to be reachable.
    marked: bool,
}; 