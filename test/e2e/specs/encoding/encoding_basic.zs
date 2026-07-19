import { hexEncode, hexDecode, base64Encode, base64Decode } from "@std/encoding";

// Distinct sentinels; 0 = all pass.
function main(): i32 {
  if (!hexEncode("abc").equals("616263")) { return 1; }
  if (!base64Encode("hello").equals("aGVsbG8=")) { return 2; }
  if (!base64Encode("abc").equals("YWJj")) { return 3; }

  let hs: string = hexDecode("48656c6c6f");
  if (!hs.equals("Hello")) { return 4; }

  let bs: string = base64Decode("YWJj");
  if (!bs.equals("abc")) { return 5; }

  // empty inputs
  if (!hexEncode("").equals("")) { return 6; }
  if (!base64Encode("").equals("")) { return 7; }

  return 0;
}
