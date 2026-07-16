// Number primordial — the boxed form of a numeric primitive (i8..i64, f32, f64), stored as an f64.
// It exists so a scalar can be given an object identity at object boundaries: calling a method on a
// number, assigning a number into an interface slot, etc. This box is created only by the autobox
// lowering (BOX -> ALLOC_OBJ + store), never by arithmetic — value semantics stay on the unboxed
// scalar, so the numeric fast path is untouched. Methods forward to zeus_number_*.
class Number {
    private value: f64;
    public extern("zeus_number_toString") toString(): string;
}
