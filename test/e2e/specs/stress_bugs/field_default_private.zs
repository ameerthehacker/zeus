// BUG (wiki/compiler-bugs.md "Class fields default to private"): a field with no access
// modifier is inaccessible from outside the class, so this (TS-style, and matching several
// docs examples) fails. Correct today: `public name: string`.
class Person {
  name: string;
  constructor(name: string) { this.name = name; }
}
function main(): i32 {
  let p: Person = new Person("zeus");
  return (p.name.length as i32) - 4;
}
