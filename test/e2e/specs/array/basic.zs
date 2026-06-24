function main(): u32 {
  let array: u8[] = new u8[];

  array.push(127);
  array.push(125);

  return array.get(0) - array.get(1);
}
