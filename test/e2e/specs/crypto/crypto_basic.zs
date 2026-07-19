import { Hash, sha256, sha1, md5, createHash, hmacSha256, pbkdf2Sync, randomBytes, randomInt, randomUUID, timingSafeEqual } from "@std/crypto";
import { hexEncode } from "@std/encoding";

// Known-answer vectors + randomness sanity. Distinct sentinels; 0 = all pass.
function main(): i32 {
  if (!sha256("abc").equals("ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad")) { return 1; }
  if (!sha1("abc").equals("a9993e364706816aba3e25717850c26c9cd0d89d")) { return 2; }
  if (!md5("abc").equals("900150983cd24fb0d6963f7d28e17f72")) { return 3; }

  // streaming equals one-shot
  if (!createHash("sha256").update("a").update("bc").digestHex().equals(sha256("abc"))) { return 4; }

  // HMAC-SHA256 RFC 4231 case 2
  if (!hmacSha256("Jefe", "what do ya want for nothing?").equals("5bdcc146bf60754e6a042426089575c75a003f089d2739839dec58b964ec3843")) { return 5; }

  // PBKDF2 RFC 6070 (sha1, 1 iter, 20 bytes)
  if (!hexEncode(pbkdf2Sync("password", "salt", 1, 20, "sha1")).equals("0c60c80f961f0e71f3a9b524af6012062fe037a6")) { return 6; }

  if (randomBytes(16).length != 16) { return 7; }
  let r: i32 = randomInt(0, 10);
  if (r < 0 || r >= 10) { return 8; }
  let uuid: string = randomUUID();
  if (uuid.length != 36) { return 9; }
  if (!uuid.charAt(14).equals("4")) { return 10; }

  if (!timingSafeEqual("secret", "secret")) { return 11; }
  if (timingSafeEqual("secret", "secreu")) { return 12; }
  if (timingSafeEqual("abc", "abcd")) { return 13; }

  return 0;
}
