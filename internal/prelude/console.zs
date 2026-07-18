// console logging primordial. Every method is variadic (TS-style infinite args): each argument is
// converted to a string (via its toString / runtime reflection) and the runtime joins them with a
// single space + newline. log/info/debug write to stdout; warn/error write to stderr.
class Console {
    @extern("zeus_Console_log") public log(...args: string[]): void;
    @extern("zeus_Console_info") public info(...args: string[]): void;
    @extern("zeus_Console_debug") public debug(...args: string[]): void;
    @extern("zeus_Console_warn") public warn(...args: string[]): void;
    @extern("zeus_Console_error") public error(...args: string[]): void;
}
