interface Shape {
  area(): i32;
}

function main(): i32 {
  // An interface is a type, not a class — it cannot be instantiated.
  let s: Shape = new Shape();

  return 0;
}
