// string primordial — a length-prefixed u8 array. The constructor interns via the Zig runtime and
// the methods forward to zeus_string_*. Self-hosted prelude; compiled before Error/Console, which
// reference `string`. The methods are self-referential (string → string), resolved like any
// self-referential class.
class string {
    private data: u8[];
    public readonly length: i32;
    public extern("zeus_string_constructor") constructor(bytes: u8[]): void;
    public extern("zeus_string_compare") compare(other: string): i8;
    public extern("zeus_string_equals") equals(other: string): boolean;
    public extern("zeus_string_concat") concat(other: string): string;
}
