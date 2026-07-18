// Nested values render via their own toString instead of structurally: the Money field via its
// `$...`, the Number box as 5, the boolean inline; an array of toString objects joins their toString.
class Money { c: i32; constructor(c: i32) { this.c = c; } toString(): string { return "$" + this.c; } }
class Wallet {
    amount: Money;
    count: Number;
    ok: boolean;
    constructor(m: Money) { this.amount = m; this.count = 5; this.ok = true; }
}
console.log(new Wallet(new Money(500)));
console.log([new Money(1), new Money(2)]);
