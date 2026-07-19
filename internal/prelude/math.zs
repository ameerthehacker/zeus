// Math primordial — JavaScript-style math namespace. A pure static class: there is no instance and
// no singleton, so `Math.sqrt(16.0)` and `Math.PI` work with no import. The constants are static
// readonly fields initialized inline; every method forwards to a zeus_Math_* runtime function.
// Self-hosted prelude — loaded automatically from this directory.
//
// Note: `min`/`max` are binary (Zeus has no varargs) and all methods operate on f64.
class Math {
    public static readonly PI: f64 = 3.141592653589793;
    public static readonly E: f64 = 2.718281828459045;
    public static readonly LN2: f64 = 0.6931471805599453;
    public static readonly LN10: f64 = 2.302585092994046;
    public static readonly LOG2E: f64 = 1.4426950408889634;
    public static readonly LOG10E: f64 = 0.4342944819032518;
    public static readonly SQRT2: f64 = 1.4142135623730951;
    public static readonly SQRT1_2: f64 = 0.7071067811865476;

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
    @extern("zeus_Math_asin") public static   asin(x: f64): f64;
    @extern("zeus_Math_acos") public static   acos(x: f64): f64;
    @extern("zeus_Math_atan") public static   atan(x: f64): f64;
    @extern("zeus_Math_atan2") public static  atan2(y: f64, x: f64): f64;
    @extern("zeus_Math_sinh") public static   sinh(x: f64): f64;
    @extern("zeus_Math_cosh") public static   cosh(x: f64): f64;
    @extern("zeus_Math_tanh") public static   tanh(x: f64): f64;
    @extern("zeus_Math_asinh") public static  asinh(x: f64): f64;
    @extern("zeus_Math_acosh") public static  acosh(x: f64): f64;
    @extern("zeus_Math_atanh") public static  atanh(x: f64): f64;
    @extern("zeus_Math_log1p") public static  log1p(x: f64): f64;
    @extern("zeus_Math_expm1") public static  expm1(x: f64): f64;
    @extern("zeus_Math_fround") public static fround(x: f64): f64;
    @extern("zeus_Math_clz32") public static  clz32(x: f64): f64;
    @extern("zeus_Math_imul") public static   imul(a: f64, b: f64): f64;
}
