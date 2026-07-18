// A private override of a public method is rejected (would bypass access control via dispatch).
class Base { public speak(): string { return "base"; } }
class Derived extends Base { private speak(): string { return "derived"; } }
let d: Derived = new Derived();
console.log(d.speak());
