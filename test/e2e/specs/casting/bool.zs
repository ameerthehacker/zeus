// bool <-> numeric `as` casts.
function main(): i32 {
  // bool -> int gives 0/1
  if ((true as i32) != 1) { return 1; }
  if ((false as i32) != 0) { return 2; }

  // int -> bool is x != 0
  let x: i32 = 5;
  if (!(x as boolean)) { return 3; }
  let y: i32 = 0;
  if (y as boolean) { return 4; }

  // float -> bool is x != 0.0
  let f: f64 = 3.14;
  if (!(f as boolean)) { return 5; }
  let z: f64 = 0.0;
  if (z as boolean) { return 6; }

  // bool -> float gives 0.0/1.0
  if ((true as f64) != 1.0) { return 7; }

  return 0;
}
