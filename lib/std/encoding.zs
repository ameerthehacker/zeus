// @std/encoding — hex and base64 conversion between byte arrays and strings.
//
//   import { hexEncode, base64Encode } from "@std/encoding";
//   let sig: string = hexEncode(bytes);   // bytes: u8[]  (a string coerces to u8[] too)
//
// Encoders take a u8[] and return the encoded ASCII string; decoders take a string and return the
// raw u8[]. Decoders throw on malformed input. Backed by runtime/encoding_runtime.zig.

@extern("zeus", "hex_encode")    function cHexEncode(data: cptr, len: clong, out: cptr): clong;
@extern("zeus", "hex_decode")    function cHexDecode(s: cptr, len: clong, out: cptr): clong;
@extern("zeus", "base64_encode") function cB64Encode(data: cptr, len: clong, out: cptr): clong;
@extern("zeus", "base64_decode") function cB64Decode(s: cptr, len: clong, out: cptr): clong;

// hexEncode returns the lowercase hex representation of the bytes.
export function hexEncode(bytes: u8[]): string {
  let len: i64 = cBytesLen(bytes) as i64;
  if (len == 0) {
    return "";
  }
  let out: cptr = cMalloc((len * 2) as csize);
  let n: i64 = cHexEncode(cBytesPtr(bytes), len as clong, out) as i64;
  let s: string = cBytesToString(out, n as clong);
  cFree(out);
  return s;
}

// hexDecode parses a hex string into bytes. Throws on odd length or non-hex characters.
export function hexDecode(s: string): u8[] {
  if (s.length == 0) {
    return new u8[];
  }
  let out: cptr = cMalloc((s.length / 2 + 1) as csize);
  let n: i64 = cHexDecode(cStrFromString(s) as cptr, s.length as clong, out) as i64;
  if (n < 0) {
    cFree(out);
    throw new Error("EncodingError", "invalid hex string");
  }
  let bytes: string = cBytesToString(out, n as clong);
  cFree(out);
  return bytes;
}

// base64Encode returns the standard (padded) base64 representation of the bytes.
export function base64Encode(bytes: u8[]): string {
  let len: i64 = cBytesLen(bytes) as i64;
  if (len == 0) {
    return "";
  }
  let cap: i64 = ((len + 2) / 3) * 4 + 4;
  let out: cptr = cMalloc(cap as csize);
  let n: i64 = cB64Encode(cBytesPtr(bytes), len as clong, out) as i64;
  let s: string = cBytesToString(out, n as clong);
  cFree(out);
  return s;
}

// base64Decode parses a standard base64 string into bytes. Throws on malformed input.
export function base64Decode(s: string): u8[] {
  if (s.length == 0) {
    return new u8[];
  }
  let out: cptr = cMalloc((s.length + 1) as csize);
  let n: i64 = cB64Decode(cStrFromString(s) as cptr, s.length as clong, out) as i64;
  if (n < 0) {
    cFree(out);
    throw new Error("EncodingError", "invalid base64 string");
  }
  let bytes: string = cBytesToString(out, n as clong);
  cFree(out);
  return bytes;
}
