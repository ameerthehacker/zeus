// Exercises the JS/TS-parity string methods. Each failure returns a distinct sentinel; 0 = all pass.
function main(): i32 {
  let s: string = "Hello, World";

  if (!s.slice(0, 5).equals("Hello")) { return 1; }
  if (!s.slice(-5, s.length).equals("World")) { return 2; }
  if (!s.substring(7, 12).equals("World")) { return 3; }
  if (!s.substring(12, 7).equals("World")) { return 4; }   // swaps args
  if (!s.toUpperCase().equals("HELLO, WORLD")) { return 5; }
  if (!s.toLowerCase().equals("hello, world")) { return 6; }
  if (!"  x y  ".trim().equals("x y")) { return 7; }
  if (!"  x  ".trimStart().equals("x  ")) { return 8; }
  if (!"  x  ".trimEnd().equals("  x")) { return 9; }
  if (!"ab".repeat(3).equals("ababab")) { return 10; }
  if (!"ab".repeat(0).equals("")) { return 11; }
  if (!"7".padStart(3, "0").equals("007")) { return 12; }
  if (!"7".padEnd(3, ".").equals("7..")) { return 13; }
  if (!"a,b,c".replace(",", "-").equals("a-b,c")) { return 14; }
  if (!"a,b,c".replaceAll(",", "-").equals("a-b-c")) { return 15; }
  if (!s.charAt(1).equals("e")) { return 16; }
  if (!s.charAt(100).equals("")) { return 17; }

  if (s.indexOf("World") != 7) { return 18; }
  if (s.indexOf("zzz") != -1) { return 19; }
  if (s.lastIndexOf("o") != 8) { return 20; }
  if (s.charCodeAt(0) != 72) { return 21; }
  if (s.charCodeAt(999) != -1) { return 22; }

  if (!s.includes("World")) { return 23; }
  if (s.includes("world")) { return 24; }          // case-sensitive
  if (!s.startsWith("Hello")) { return 25; }
  if (!s.endsWith("World")) { return 26; }

  return 0;
}
