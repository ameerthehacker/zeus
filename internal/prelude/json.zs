// JSON primordials — global (no import), matching TypeScript's `JSON`. `JsonValue` is the tree node
// type; `JSON` is the static namespace (JSON.parse / JSON.stringify / builders). Every method is
// runtime-backed (extern): the tree lives as GC-allocated nodes reached through the JsonValue.node
// pointer, so the collector traces the whole document while a JsonValue is reachable.
//
// Because Zeus has no `any` type, `stringify` takes a JsonValue (built via JSON.newObject/… or from
// parse) rather than an arbitrary native object.
class JsonValue {
    // The tree node pointer, stored as i64 (a cptr field would break reflection metadata). It is
    // byte-identical to the pointer the runtime writes, and Boehm still scans it conservatively, so
    // the GC-allocated node tree stays reachable through this field.
    private node: i64;

    @extern("zeus_JsonValue_kind")       public kind(): i32;
    @extern("zeus_JsonValue_isNull")     public isNull(): boolean;
    @extern("zeus_JsonValue_isBool")     public isBool(): boolean;
    @extern("zeus_JsonValue_isNumber")   public isNumber(): boolean;
    @extern("zeus_JsonValue_isString")   public isString(): boolean;
    @extern("zeus_JsonValue_isArray")    public isArray(): boolean;
    @extern("zeus_JsonValue_isObject")   public isObject(): boolean;
    @extern("zeus_JsonValue_asBool")     public asBool(): boolean;
    @extern("zeus_JsonValue_asNumber")   public asNumber(): f64;
    @extern("zeus_JsonValue_asInt")      public asInt(): i32;
    @extern("zeus_JsonValue_asString")   public asString(): string;
    @extern("zeus_JsonValue_length")     public length(): i32;
    @extern("zeus_JsonValue_at")         public at(index: i32): JsonValue;
    @extern("zeus_JsonValue_has")        public has(key: string): boolean;
    @extern("zeus_JsonValue_get")        public get(key: string): JsonValue;
    @extern("zeus_JsonValue_objectKeys") public objectKeys(): string[];
    @extern("zeus_JsonValue_push")       public push(value: JsonValue): void;
    @extern("zeus_JsonValue_set")        public set(key: string, value: JsonValue): void;
}

class JSON {
    @extern("zeus_JSON_parse")     public static parse(text: string): JsonValue;
    @extern("zeus_JSON_stringify") public static stringify(value: JsonValue): string;
    @extern("zeus_JSON_newObject") public static newObject(): JsonValue;
    @extern("zeus_JSON_newArray")  public static newArray(): JsonValue;
    @extern("zeus_JSON_newString") public static newString(s: string): JsonValue;
    @extern("zeus_JSON_newNumber") public static newNumber(n: f64): JsonValue;
    @extern("zeus_JSON_newBool")   public static newBool(b: boolean): JsonValue;
    @extern("zeus_JSON_newNull")   public static newNull(): JsonValue;
}
