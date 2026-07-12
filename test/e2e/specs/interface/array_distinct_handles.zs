// Two different object-array types conform to the same interface and are dispatched through one
// interface-typed site. Each carries its OWN class id (distinct type handle), so the itable keys
// them independently even though they share the Object[] code — a collision would misdispatch.
class A { public v: i32; constructor(v: i32) { this.v = v; } }
class B { public v: i32; constructor(v: i32) { this.v = v; } }

interface Sized {
  readonly length: i32;
}

function sz(s: Sized): i32 {
  return s.length;
}

function main(): i32 {
  let a: A[] = new A[];
  a.push(new A(1));

  let b: B[] = new B[];
  b.push(new B(1));
  b.push(new B(2));
  b.push(new B(3));

  return sz(a) * 10 + sz(b); // 1*10 + 3 = 13
}
