// Math primordial — JavaScript-style math namespace. A pure static class: there is no instance and
// no singleton, so `Math.sqrt(16.0)` and `Math.PI` work with no import. The constants are static
// readonly fields initialized inline; every method forwards to a zeus_Math_* runtime function.
// Self-hosted prelude — loaded automatically from this directory.
//
// Note: `min`/`max` are binary (Zeus has no varargs) and all methods operate on f64.
class Math {
    public static readonly PI: f64 = 3.141592653589793;
    public static readonly E: f64 = 2.718281828459045;

    @extern("zeus_Math_sqrt") public static   sqrt(x: f64): f64;
    @extern("zeus_Math_cbrt") public static   cbrt(x: f64): f64;
    @extern("zeus_Math_pow") public static    pow(x: f64, y: f64): f64;
    @extern("zeus_Math_exp") public static    exp(x: f64): f64;
    @extern("zeus_Math_log") public static    log(x: f64): f64;
    @extern("zeus_Math_log2") public static   log2(x: f64): f64;
    @extern("zeus_Math_log10") public static  log10(x: f64): f64;
    @extern("zeus_Math_sin") public static    sin(x: f64): f64;
    @extern("zeus_Math_cos") public static    cos(x: f64): f64;
    @extern("zeus_Math_tan") public static    tan(x: f64): f64;
    @extern("zeus_Math_floor") public static  floor(x: f64): f64;
    @extern("zeus_Math_ceil") public static   ceil(x: f64): f64;
    @extern("zeus_Math_round") public static  round(x: f64): f64;
    @extern("zeus_Math_trunc") public static  trunc(x: f64): f64;
    @extern("zeus_Math_abs") public static    abs(x: f64): f64;
    @extern("zeus_Math_sign") public static   sign(x: f64): f64;
    @extern("zeus_Math_min") public static    min(a: f64, b: f64): f64;
    @extern("zeus_Math_max") public static    max(a: f64, b: f64): f64;
    @extern("zeus_Math_hypot") public static  hypot(a: f64, b: f64): f64;
    @extern("zeus_Math_random") public static random(): f64;
}
