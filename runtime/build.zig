const std = @import("std");

pub fn build(b: *std.Build) void {
    var target_query = b.standardTargetOptionsQueryOnly(.{});

    // Set minimum macOS deployment target to avoid linker warnings
    if (target_query.os_tag == null or target_query.os_tag.? == .macos) {
        target_query.os_version_min = .{ .semver = .{ .major = 12, .minor = 0, .patch = 0 } };
    }

    const target = b.resolveTargetQuery(target_query);
    const optimize = b.option(std.builtin.OptimizeMode, "optimize", "Optimization mode") orelse .ReleaseFast;

    // Paths for the Boehm GC (third_party/bdwgc built by CMakeLists.txt)
    const gc_include = b.option([]const u8, "gc_include", "Path to bdwgc headers") orelse "";
    const gc_lib = b.option([]const u8, "gc_lib", "Path to bdwgc lib dir") orelse "";

    // libxev dependency (build.zig.zon declares the package)
    const xev_dep = b.dependency("xev", .{ .target = target, .optimize = optimize });
    const xev_module = xev_dep.module("xev");

    const output_dir = "out";

    const runtime_lib = b.addStaticLibrary(.{
        .name = "zeus-runtime",
        .root_source_file = b.path("main.zig"),
        .target = target,
        .optimize = optimize,
    });
    applyRuntimeSettings(runtime_lib, xev_module, gc_include, gc_lib);

    const install_lib = b.addInstallArtifact(runtime_lib, .{
        .dest_dir = .{ .override = .{ .custom = output_dir } },
    });

    const runtime_obj = b.addObject(.{
        .name = "zeus-runtime",
        .root_source_file = b.path("main.zig"),
        .target = target,
        .optimize = optimize,
    });
    applyRuntimeSettings(runtime_obj, xev_module, gc_include, gc_lib);

    const install_obj = b.addInstallArtifact(runtime_obj, .{
        .dest_dir = .{ .override = .{ .custom = output_dir } },
    });

    b.getInstallStep().dependOn(&install_lib.step);
    b.getInstallStep().dependOn(&install_obj.step);

    const lib_step = b.step("lib", "Build the runtime as a static library");
    lib_step.dependOn(&install_lib.step);

    const obj_step = b.step("obj", "Build the runtime as an object file");
    obj_step.dependOn(&install_obj.step);

    const clean_step = b.step("clean", "Clean build artifacts");
    const clean_cmd = b.addSystemCommand(&[_][]const u8{ "rm", "-rf", output_dir, ".zig-cache" });
    clean_step.dependOn(&clean_cmd.step);

    const test_step = b.step("test", "Run runtime tests");

    const test_main = b.addTest(.{
        .root_source_file = b.path("main.zig"),
        .target = target,
        .optimize = optimize,
    });
    applyRuntimeSettings(test_main, xev_module, gc_include, gc_lib);

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

fn applyRuntimeSettings(
    step: *std.Build.Step.Compile,
    xev_module: *std.Build.Module,
    gc_include: []const u8,
    gc_lib: []const u8,
) void {
    step.linkLibC();
    step.linkSystemLibrary("unwind");
    step.root_module.addImport("xev", xev_module);
    if (gc_include.len > 0) step.addIncludePath(.{ .cwd_relative = gc_include });
    if (gc_lib.len > 0) step.addLibraryPath(.{ .cwd_relative = gc_lib });
    step.linkSystemLibrary("gc");
}
