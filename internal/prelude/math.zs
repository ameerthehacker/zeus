// Math primordial — JavaScript-style math namespace. A pure static class: there is no instance and
// no singleton, so `Math.sqrt(16.0)` and `Math.PI` work with no import. The constants are static
// readonly fields initialized inline; every method forwards to a zeus_Math_* runtime function.
// Self-hosted prelude — loaded automatically from this directory.
//
// Note: `min`/`max` are binary (Zeus has no varargs) and all methods operate on f64.
class Math {
    public static readonly PI: f64 = 3.141592653589793;
    public static readonly E: f64 = 2.718281828459045;

    public static extern("zeus_Math_sqrt")   sqrt(x: f64): f64;
    public static extern("zeus_Math_cbrt")   cbrt(x: f64): f64;
    public static extern("zeus_Math_pow")    pow(x: f64, y: f64): f64;
    public static extern("zeus_Math_exp")    exp(x: f64): f64;
    public static extern("zeus_Math_log")    log(x: f64): f64;
    public static extern("zeus_Math_log2")   log2(x: f64): f64;
    public static extern("zeus_Math_log10")  log10(x: f64): f64;
    public static extern("zeus_Math_sin")    sin(x: f64): f64;
    public static extern("zeus_Math_cos")    cos(x: f64): f64;
    public static extern("zeus_Math_tan")    tan(x: f64): f64;
    public static extern("zeus_Math_floor")  floor(x: f64): f64;
    public static extern("zeus_Math_ceil")   ceil(x: f64): f64;
    public static extern("zeus_Math_round")  round(x: f64): f64;
    public static extern("zeus_Math_trunc")  trunc(x: f64): f64;
    public static extern("zeus_Math_abs")    abs(x: f64): f64;
    public static extern("zeus_Math_sign")   sign(x: f64): f64;
    public static extern("zeus_Math_min")    min(a: f64, b: f64): f64;
    public static extern("zeus_Math_max")    max(a: f64, b: f64): f64;
    public static extern("zeus_Math_hypot")  hypot(a: f64, b: f64): f64;
    public static extern("zeus_Math_random") random(): f64;
}
