// Error primordial — base class for all exceptions. Its reserved class ID is stamped at load time
// (bootstrapPreludes), and its constructor forwards to the Zig runtime. Self-hosted prelude.
class Error {
    public name: string;
    public message: string;
    @extern("zeus_error_constructor") public constructor(name: string, msg: string): void;
}
