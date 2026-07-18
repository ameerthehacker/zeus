// Stringify — the ambient interface for "can be turned into a string". Any type that declares
// `toString(): string` satisfies it structurally (the boxed primitives I8..F64 and Bool, the Number
// umbrella, `string` itself, and user classes). It powers the implicit `T -> string` conversion:
// wherever a `string` is expected, a Stringify value is converted by calling toString() — so
// `console.log(5)`, `let s: string = obj`, and `"n=" + x` work. Interfaces emit no IR; loadPreludes
// harvests this from the compiled prelude symbol table and injects it into every module.
interface Stringify {
    toString(): string;
}
