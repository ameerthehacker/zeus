// Global number parsing/predicates — the JS globals parseInt/parseFloat/isNaN/isFinite. Ambient
// primordial free functions (like the timer functions), each forwarding to its zeus_* runtime
// symbol. parseInt/parseFloat return f64 (NaN on failure) to match JS's Number-typed result; parse
// is lenient (leading numeric prefix, JS-style), with parseInt auto-detecting a 0x hex prefix.
@extern("zeus", "parse_int")   function parseInt(s: string): f64;
@extern("zeus", "parse_float") function parseFloat(s: string): f64;
@extern("zeus", "is_nan")      function isNaN(x: f64): boolean;
@extern("zeus", "is_finite")   function isFinite(x: f64): boolean;
