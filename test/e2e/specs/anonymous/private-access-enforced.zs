class Wallet {
  private balance: i32;
  constructor(b: i32) { this.balance = b; }
  getBalance(): i32 { return this.balance; }
}

function main(): i32 {
  let w = new Wallet(42);
  return w.getBalance();
}
