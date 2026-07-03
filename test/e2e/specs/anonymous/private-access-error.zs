class Wallet {
  private balance: i32;
  constructor(b: i32) { this.balance = b; }
}

function main(): i32 {
  let w = new Wallet(42);
  return w.balance;
}
