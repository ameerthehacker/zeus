// string primordial — a length-prefixed u8 array. The constructor interns via the Zig runtime and
// the methods forward to zeus_string_*. Self-hosted prelude; compiled before Error/Console, which
// reference `string`. The methods are self-referential (string → string), resolved like any
// self-referential class.
class string {
    private data: u8[];
    public readonly length: i32;
    @extern("zeus_string_constructor") public constructor(bytes: u8[]): void;
    @extern("zeus_string_compare") public compare(other: string): i8;
    @extern("zeus_string_equals") public equals(other: string): boolean;
    @extern("zeus_string_concat") public concat(other: string): string;
}
