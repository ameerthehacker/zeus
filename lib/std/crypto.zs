// @std/crypto — synchronous cryptography, entirely on Zig's std.crypto (no OpenSSL). A subset of
// Node's crypto module.
//
//   import { sha256, createHash, hmacSha256, randomUUID } from "@std/crypto";
//   console.log(sha256("hello"));                 // hex digest
//   console.log(createHash("sha512").update("a").update("b").digestHex());
//
// Digest/HMAC/PBKDF2 helpers take a u8[] (a string coerces to its UTF-8 bytes) and return a lowercase
// hex string; the raw-bytes and base64 forms are available too. Zeus has no cross-module static
// methods yet, so Node's crypto.createHash is the free function createHash, etc.

import { hexEncode, base64Encode } from "@std/encoding";

@extern("zeus", "crypto_hash")   function cCryptoHash(algo: cint, data: cptr, len: clong, out: cptr): clong;
@extern("zeus", "crypto_hmac")   function cCryptoHmac(algo: cint, key: cptr, keyLen: clong, data: cptr, dataLen: clong, out: cptr): clong;
@extern("zeus", "crypto_pbkdf2") function cCryptoPbkdf2(algo: cint, pw: cptr, pwLen: clong, salt: cptr, saltLen: clong, iters: cint, keylen: cint, out: cptr): clong;
@extern("zeus", "crypto_random_bytes") function cCryptoRandomBytes(out: cptr, n: cint): void;
@extern("zeus", "crypto_random_int")   function cCryptoRandomInt(min: cint, max: cint): cint;
@extern("zeus", "crypto_random_uuid")  function cCryptoRandomUuid(out: cptr): void;
@extern("zeus", "crypto_timing_safe_equal") function cCryptoTimingSafeEqual(a: cptr, aLen: clong, b: cptr, bLen: clong): cint;

const ALGO_MD5: i32 = 0;
const ALGO_SHA1: i32 = 1;
const ALGO_SHA224: i32 = 2;
const ALGO_SHA256: i32 = 3;
const ALGO_SHA384: i32 = 4;
const ALGO_SHA512: i32 = 5;
const MAX_DIGEST: csize = 64 as csize; // sha512

// algoCode maps an algorithm name to its runtime code, throwing on an unknown name.
function algoCode(name: string): i32 {
  if (name.equals("md5")) { return ALGO_MD5; }
  if (name.equals("sha1")) { return ALGO_SHA1; }
  if (name.equals("sha224")) { return ALGO_SHA224; }
  if (name.equals("sha256")) { return ALGO_SHA256; }
  if (name.equals("sha384")) { return ALGO_SHA384; }
  if (name.equals("sha512")) { return ALGO_SHA512; }
  throw new Error("CryptoError", "unsupported algorithm: " + name);
}

// hashRaw returns the raw digest bytes for `data` under the given algorithm code.
function hashRaw(algo: i32, data: u8[]): u8[] {
  let out: cptr = cMalloc(MAX_DIGEST);
  let n: i64 = cCryptoHash(algo as cint, cBytesPtr(data), cBytesLen(data), out) as i64;
  let bytes: string = cBytesToString(out, n as clong);
  cFree(out);
  return bytes;
}

// hmacRaw returns the raw HMAC bytes of `data` under `key`.
function hmacRaw(algo: i32, key: u8[], data: u8[]): u8[] {
  let out: cptr = cMalloc(MAX_DIGEST);
  let n: i64 = cCryptoHmac(algo as cint, cBytesPtr(key), cBytesLen(key), cBytesPtr(data), cBytesLen(data), out) as i64;
  let bytes: string = cBytesToString(out, n as clong);
  cFree(out);
  return bytes;
}

// Hash accumulates input across update() calls, then produces a digest. Mirrors Node's Hash.
export class Hash {
  private algo: i32;
  private buf: u8[];

  public constructor(algorithm: string) {
    this.algo = algoCode(algorithm);
    this.buf = new u8[];
  }

  // update appends more data and returns the Hash for chaining.
  public update(data: u8[]): Hash {
    let i: i32 = 0;
    while (i < data.length) {
      this.buf.push(data[i]);
      i = i + 1;
    }
    return this;
  }

  public digestBytes(): u8[] {
    return hashRaw(this.algo, this.buf);
  }

  public digestHex(): string {
    return hexEncode(hashRaw(this.algo, this.buf));
  }

  public digestBase64(): string {
    return base64Encode(hashRaw(this.algo, this.buf));
  }
}

// createHash returns a Hash for the named algorithm ("sha256", "sha512", "md5", ...).
export function createHash(algorithm: string): Hash {
  return new Hash(algorithm);
}

// One-shot hex digests.
export function md5(data: u8[]): string {
  return hexEncode(hashRaw(ALGO_MD5, data));
}

export function sha1(data: u8[]): string {
  return hexEncode(hashRaw(ALGO_SHA1, data));
}

export function sha256(data: u8[]): string {
  return hexEncode(hashRaw(ALGO_SHA256, data));
}

export function sha512(data: u8[]): string {
  return hexEncode(hashRaw(ALGO_SHA512, data));
}

// HMAC hex digests.
export function hmacSha256(key: u8[], data: u8[]): string {
  return hexEncode(hmacRaw(ALGO_SHA256, key, data));
}

export function hmacSha512(key: u8[], data: u8[]): string {
  return hexEncode(hmacRaw(ALGO_SHA512, key, data));
}

// pbkdf2Sync derives `keylen` bytes from a password + salt (Node's crypto.pbkdf2Sync). `digest` is
// the underlying HMAC hash name ("sha256", ...).
export function pbkdf2Sync(password: string, salt: string, iterations: i32, keylen: i32, digest: string): u8[] {
  let algo: i32 = algoCode(digest);
  let pw: u8[] = password;
  let saltBytes: u8[] = salt;
  let out: cptr = cMalloc(keylen as csize);
  let n: i64 = cCryptoPbkdf2(algo as cint, cBytesPtr(pw), cBytesLen(pw), cBytesPtr(saltBytes), cBytesLen(saltBytes), iterations as cint, keylen as cint, out) as i64;
  if (n < 0) {
    cFree(out);
    throw new Error("CryptoError", "pbkdf2 failed");
  }
  let bytes: string = cBytesToString(out, n as clong);
  cFree(out);
  return bytes;
}

// randomBytes returns `n` cryptographically-secure random bytes.
export function randomBytes(n: i32): u8[] {
  if (n <= 0) {
    return new u8[];
  }
  let out: cptr = cMalloc(n as csize);
  cCryptoRandomBytes(out, n as cint);
  let bytes: string = cBytesToString(out, n as clong);
  cFree(out);
  return bytes;
}

// randomInt returns a secure random integer in [min, max).
export function randomInt(min: i32, max: i32): i32 {
  return cCryptoRandomInt(min as cint, max as cint) as i32;
}

// randomUUID returns a random RFC 4122 v4 UUID string.
export function randomUUID(): string {
  let out: cptr = cMalloc(40 as csize);
  cCryptoRandomUuid(out);
  let s: string = cBytesToString(out, 36 as clong);
  cFree(out);
  return s;
}

// timingSafeEqual compares two byte arrays in constant time (false if lengths differ).
export function timingSafeEqual(a: u8[], b: u8[]): boolean {
  return (cCryptoTimingSafeEqual(cBytesPtr(a), cBytesLen(a), cBytesPtr(b), cBytesLen(b)) as i32) != 0;
}
