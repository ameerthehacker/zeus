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
    // Makes `string` a Stringify (returns itself), so a string flows into a `Stringify` slot.
    @extern("zeus_string_toString") public toString(): string;

    // JS/TS-parity methods (byte-oriented, matching `string` indexing). `slice` takes JS-style
    // negative indices; pass `s.length` for "to the end".
    @extern("zeus_string_slice")       public slice(start: i32, end: i32): string;
    @extern("zeus_string_substring")   public substring(start: i32, end: i32): string;
    @extern("zeus_string_indexOf")     public indexOf(needle: string): i32;
    @extern("zeus_string_lastIndexOf") public lastIndexOf(needle: string): i32;
    @extern("zeus_string_includes")    public includes(needle: string): boolean;
    @extern("zeus_string_startsWith")  public startsWith(prefix: string): boolean;
    @extern("zeus_string_endsWith")    public endsWith(suffix: string): boolean;
    @extern("zeus_string_toUpperCase") public toUpperCase(): string;
    @extern("zeus_string_toLowerCase") public toLowerCase(): string;
    @extern("zeus_string_trim")        public trim(): string;
    @extern("zeus_string_trimStart")   public trimStart(): string;
    @extern("zeus_string_trimEnd")     public trimEnd(): string;
    @extern("zeus_string_repeat")      public repeat(count: i32): string;
    @extern("zeus_string_padStart")    public padStart(targetLength: i32, pad: string): string;
    @extern("zeus_string_padEnd")      public padEnd(targetLength: i32, pad: string): string;
    @extern("zeus_string_replace")     public replace(search: string, replacement: string): string;
    @extern("zeus_string_replaceAll")  public replaceAll(search: string, replacement: string): string;
    @extern("zeus_string_charAt")      public charAt(index: i32): string;
    @extern("zeus_string_charCodeAt")  public charCodeAt(index: i32): i32;
    @extern("zeus_string_split")       public split(separator: string): string[];
}
