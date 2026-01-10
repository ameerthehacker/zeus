function main(): i32 {
  let hello: string = "Hello";
  let empty: string = "";
  let emoji: string = "👋";

  // "Hello" has 5 bytes
  // empty string has 0 bytes
  // emoji has 4 bytes (UTF-8 encoded)
  let total: i32 = hello.length + empty.length + emoji.length;
  return total;
}
