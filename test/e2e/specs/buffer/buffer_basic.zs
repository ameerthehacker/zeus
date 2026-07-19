import { Buffer, alloc, fromString, fromHex, fromBase64, concat } from "@std/buffer";

// Distinct sentinels; 0 = all pass.
function main(): i32 {
  let b: Buffer = fromString("hello");
  if (!b.toHex().equals("68656c6c6f")) { return 1; }
  if (!b.toBase64().equals("aGVsbG8=")) { return 2; }
  if (b.length() != 5) { return 3; }
  if (b.readUInt8(0) != 104) { return 4; }

  let z: Buffer = alloc(8);
  z.writeUInt16LE(4660, 0);
  if (z.readUInt16LE(0) != 4660) { return 5; }
  z.writeUInt32LE(305419896 as i64, 4);
  if (z.readUInt32LE(4) != 305419896) { return 6; }

  if (!fromHex("616263").toString().equals("abc")) { return 7; }
  if (!fromBase64("YWJj").toString().equals("abc")) { return 8; }

  let ab: Buffer = concat(fromString("ab"), fromString("cd"));
  if (!ab.toString().equals("abcd")) { return 9; }
  if (!ab.slice(1, 3).toString().equals("bc")) { return 10; }
  if (!fromString("xy").equals(fromString("xy"))) { return 11; }
  if (fromString("xy").equals(fromString("xz"))) { return 12; }

  return 0;
}
