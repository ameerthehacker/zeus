// ANSI Color utilities for terminal output
// Provides consistent color formatting across the runtime
// Supports NO_COLOR environment variable (https://no-color.org/)

const std = @import("std");

/// Check if colors should be disabled (NO_COLOR env var is set)
var no_color_checked: bool = false;
var no_color_enabled: bool = false;

fn isNoColor() bool {
    if (!no_color_checked) {
        no_color_checked = true;
        if (std.posix.getenv("NO_COLOR")) |_| {
            no_color_enabled = true;
        }
    }
    return no_color_enabled;
}

/// Get color code, returning empty string if NO_COLOR is set
pub fn get(comptime color: []const u8) []const u8 {
    if (isNoColor()) return "";
    return color;
}

/// ANSI Reset - clears all formatting
pub const reset = "\x1b[0m";

/// Bold text
pub const bold = "\x1b[1m";

/// Colors
pub const red = "\x1b[31m";
pub const green = "\x1b[32m";
pub const yellow = "\x1b[33m";
pub const blue = "\x1b[34m";
pub const magenta = "\x1b[35m";
pub const cyan = "\x1b[36m";
pub const white = "\x1b[37m";
pub const gray = "\x1b[90m";

/// Bold colors (commonly used combinations)
pub const red_bold = "\x1b[31;1m";
pub const green_bold = "\x1b[32;1m";
pub const yellow_bold = "\x1b[33;1m";
pub const blue_bold = "\x1b[34;1m";
pub const magenta_bold = "\x1b[35;1m";
pub const cyan_bold = "\x1b[36;1m";
pub const white_bold = "\x1b[37;1m";

/// Format a string with color and automatic reset
pub fn colorize(comptime color: []const u8, text: []const u8) void {
    const stderr = std.io.getStdErr().writer();
    if (isNoColor()) {
        stderr.print("{s}", .{text}) catch {};
    } else {
        stderr.print("{s}{s}{s}", .{ color, text, reset }) catch {};
    }
}

/// Print formatted text with color
pub fn printColored(comptime color: []const u8, comptime fmt: []const u8, args: anytype) void {
    const stderr = std.io.getStdErr().writer();
    if (isNoColor()) {
        stderr.print(fmt, args) catch {};
    } else {
        stderr.print(color, .{}) catch {};
        stderr.print(fmt, args) catch {};
        stderr.print(reset, .{}) catch {};
    }
}
