// Math gaps (inverse/hyperbolic trig, log1p/expm1, clz32/imul/fround, constants) plus the global
// number parsers (parseInt/parseFloat/isNaN/isFinite). Distinct sentinels; 0 = all pass.
function main(): i32 {
  if (Math.acos(1.0) != 0.0) { return 1; }
  if (Math.asin(0.0) != 0.0) { return 2; }
  if (Math.atan(0.0) != 0.0) { return 3; }
  if (Math.atan2(0.0, 1.0) != 0.0) { return 4; }
  if (Math.sinh(0.0) != 0.0) { return 5; }
  if (Math.cosh(0.0) != 1.0) { return 6; }
  if (Math.tanh(0.0) != 0.0) { return 7; }
  if (Math.log1p(0.0) != 0.0) { return 8; }
  if (Math.expm1(0.0) != 0.0) { return 9; }
  if (Math.clz32(1.0) != 31.0) { return 10; }
  if (Math.imul(3.0, 4.0) != 12.0) { return 11; }
  if (Math.fround(1.0) != 1.0) { return 12; }
  if (Math.SQRT2 < 1.41 || Math.SQRT2 > 1.42) { return 13; }
  if (Math.LN2 < 0.69 || Math.LN2 > 0.70) { return 14; }

  if (parseInt("42") != 42.0) { return 15; }
  if (parseInt("  -17px") != -17.0) { return 16; }
  if (parseInt("0xFF") != 255.0) { return 17; }
  if (parseFloat("3.14abc") != 3.14) { return 18; }
  if (parseFloat("1e3") != 1000.0) { return 19; }

  if (!isNaN(parseInt("hello"))) { return 20; }
  if (isNaN(parseInt("5"))) { return 21; }
  if (!isFinite(1.0)) { return 22; }
  if (isFinite(parseFloat("nope"))) { return 23; }

  return 0;
}
