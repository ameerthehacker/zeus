// BUG/gotcha (wiki "Arrays are not covariant"): C[] is not assignable to Interface[] even
// though C implements the interface. When/if made covariant, this returns 0.
interface D { d(): i32; }
class C { public v: i32; constructor(v: i32) { this.v = v; } public d(): i32 { return this.v; } }
function sum(items: D[]): i32 { let t: i32 = 0; let i: i32 = 0; while (i < (items.length as i32)) { t = t + items[i].d(); i = i + 1; } return t; }
function main(): i32 {
  let a: C[] = new C[];
  a.push(new C(3)); a.push(new C(4));
  return sum(a) - 7;
}
