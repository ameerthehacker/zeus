interface Named {
  id: i32;
}

// `id` is the second field, so its byte offset is non-trivial — the property dispatch
// table must resolve it correctly through the interface.
class Person {
  public age: i32;
  public id: i32;
  constructor(a: i32, i: i32) { this.age = a; this.id = i; }
}

function getId(n: Named): i32 {
  return n.id;
}

function main(): i32 {
  let p: Person = new Person(10, 42);
  return getId(p);
}
