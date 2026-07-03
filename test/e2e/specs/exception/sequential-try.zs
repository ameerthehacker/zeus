function main(): i32 {
  let caught1: i32 = 0;
  let caught2: i32 = 0;

  try {
    throw new Error("Error", "first");
    caught1 = 99;
  } catch (e: Error) {
    caught1 = 1;
  }

  try {
    throw new Error("Error", "second");
    caught2 = 99;
  } catch (e: Error) {
    caught2 = 1;
  }

  if (caught1 == 1 && caught2 == 1) {
    return 0;
  }
  return 1;
}
