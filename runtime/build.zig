// Zeus Runtime Build Configuration
//
// This build.zig file configures the compilation of the Zeus garbage collector runtime.
// It replaces the previous makefile-based build configuration with a proper Zig build system.
//
// Available build commands:
//   zig build           - Build both static library and object file (default)
//   zig build lib       - Build only the static library
//   zig build obj       - Build only the object file
//   zig build test      - Run all runtime tests
//   zig build clean     - Clean build artifacts
//
// Build options:
//   -Dtarget=...        - Target architecture (default: native)
//   -Doptimize=...      - Optimization level (Debug, ReleaseSafe, ReleaseFast, ReleaseSmall)
//
// The runtime requires:
//   - libunwind (for stack walking)
//   - libc (for system integration)
//   - Frame pointers enabled (for accurate stack traces)

const std = @import("std");

pub fn build(b: *std.Build) void {
    // Standard target options allows the person running `zig build` to choose
    // what target to build for. Here we do not override the defaults, which
    // means any target is allowed, and the default is native.
    var target_query = b.standardTargetOptionsQueryOnly(.{});
    
    // Set minimum macOS deployment target to avoid linker warnings
    if (target_query.os_tag == null or target_query.os_tag.? == .macos) {
        target_query.os_version_min = .{ .semver = .{ .major = 12, .minor = 0, .patch = 0 } };
    }
    
    const target = b.resolveTargetQuery(target_query);

    // Standard optimization options allow the person running `zig build` to select
    // between Debug, ReleaseSafe, ReleaseFast, and ReleaseSmall. Here we do not
    // set a preferred release mode, allowing the user to decide how to optimize.
    const optimize = b.standardOptimizeOption(.{});

    const output_dir = "out";
    // Create the runtime static library
    const runtime_lib = b.addStaticLibrary(.{
        .name = "zeus-runtime",
        .root_source_file = b.path("main.zig"),
        .target = target,
        .optimize = optimize,
    });

    // Link with system libraries
    runtime_lib.linkLibC();
    runtime_lib.linkSystemLibrary("unwind");

    // Install the library to the output directory
    const install_lib = b.addInstallArtifact(runtime_lib, .{
        .dest_dir = .{ .override = .{ .custom = output_dir } },
    });

    // Also create an object file for compatibility with existing build
    const runtime_obj = b.addObject(.{
        .name = "zeus-runtime",
        .root_source_file = b.path("main.zig"),
        .target = target,
        .optimize = optimize,
    });

    // Apply same settings to object file
    runtime_obj.linkLibC();
    runtime_obj.linkSystemLibrary("unwind");

    // Install the object file
    const install_obj = b.addInstallArtifact(runtime_obj, .{
        .dest_dir = .{ .override = .{ .custom = output_dir } },
    });

    // Make the default build step
    b.getInstallStep().dependOn(&install_lib.step);
    b.getInstallStep().dependOn(&install_obj.step);

    // Create individual build steps
    const lib_step = b.step("lib", "Build the runtime as a static library");
    lib_step.dependOn(&install_lib.step);

    const obj_step = b.step("obj", "Build the runtime as an object file");
    obj_step.dependOn(&install_obj.step);

    // Clean step
    const clean_step = b.step("clean", "Clean build artifacts");
    const clean_cmd = b.addSystemCommand(&[_][]const u8{
        "rm", "-rf", output_dir, ".zig-cache"
    });
    clean_step.dependOn(&clean_cmd.step);

    // Test step
    const test_step = b.step("test", "Run runtime tests");
    
    // Add tests for each module
    const test_main = b.addTest(.{
        .root_source_file = b.path("main.zig"),
        .target = target,
        .optimize = optimize,
    });
    test_main.linkLibC();
    test_main.linkSystemLibrary("unwind");

    const test_gc = b.addTest(.{
        .root_source_file = b.path("gc.zig"),
        .target = target,
        .optimize = optimize,
    });

    const test_stackmap = b.addTest(.{
        .root_source_file = b.path("stackmap.zig"),
        .target = target,
        .optimize = optimize,
    });
    test_stackmap.linkLibC();
    test_stackmap.linkSystemLibrary("unwind");

    const test_debug = b.addTest(.{
        .root_source_file = b.path("debug.zig"),
        .target = target,
        .optimize = optimize,
    });

    const run_tests = b.addRunArtifact(test_main);
    run_tests.step.dependOn(&b.addRunArtifact(test_gc).step);
    run_tests.step.dependOn(&b.addRunArtifact(test_stackmap).step);
    run_tests.step.dependOn(&b.addRunArtifact(test_debug).step);

    test_step.dependOn(&run_tests.step);
}
