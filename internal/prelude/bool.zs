// Bool primordial — the boxed form of a `boolean` primitive. See number.zs for the rationale; this
// box is created only by the autobox lowering, never implicitly by boolean logic. Methods forward
// to zeus_bool_*.
class Bool {
    private value: boolean;
    @extern("zeus_bool_toString") public toString(): string;
}
