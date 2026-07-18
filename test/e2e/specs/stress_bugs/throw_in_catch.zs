// REGRESSION (fixed): throwing inside a catch block used to loop forever (the caught handler
// stayed on the handler stack). Now a re-throw propagates to the next outer handler. Returns 0.
function inner(): i32 {
  try { throw new Error("A", "first"); }
  catch (e: Error) { throw new Error("B", "second"); }
}
function main(): i32 {
  try { return inner(); }
  catch (e: Error) { if (e.name == "B") { return 0; } return 5; }
}
