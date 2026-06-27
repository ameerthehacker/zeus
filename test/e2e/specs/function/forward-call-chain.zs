function main(): i8 {
  return a();
}

function a(): i8 {
  return b();
}

function b(): i8 {
  return c();
}

function c(): i8 {
  return 42;
}
