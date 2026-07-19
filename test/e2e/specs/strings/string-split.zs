// Exercises string.split -> string[]. Each failure returns a distinct sentinel; 0 = all pass.
function main(): i32 {
  let parts: string[] = "a,b,c".split(",");
  if (parts.length != 3) { return 1; }
  if (!parts[0].equals("a")) { return 2; }
  if (!parts[2].equals("c")) { return 3; }

  let empties: string[] = "one,two,,four".split(",");
  if (empties.length != 4) { return 4; }
  if (!empties[2].equals("")) { return 5; }

  let chars: string[] = "hi!".split("");
  if (chars.length != 3) { return 6; }
  if (!chars[0].equals("h")) { return 7; }

  let whole: string[] = "no-sep".split("|");
  if (whole.length != 1) { return 8; }
  if (!whole[0].equals("no-sep")) { return 9; }

  // multi-char separator
  let segs: string[] = "a::b::c".split("::");
  if (segs.length != 3) { return 10; }
  if (!segs[1].equals("b")) { return 11; }

  return 0;
}
