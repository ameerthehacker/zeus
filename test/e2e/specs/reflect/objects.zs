// Runtime reflection: any object without a user toString renders via zeus_reflect_to_string.
// Covers fields, inheritance (base fields first), a non-Stringify class, private/absent toString,
// a null field, and reflection through concat / template interpolation.
class Point { x: i32; y: i32; constructor(x: i32, y: i32) { this.x = x; this.y = y; } }
class Person { name: string; age: i32; constructor(name: string, age: i32) { this.name = name; this.age = age; } }
class Animal { legs: i32; constructor(legs: i32) { this.legs = legs; } }
class Dog extends Animal { happy: boolean; constructor(legs: i32, happy: boolean) { super(legs); this.happy = happy; } }
class Bare { public x: i32; public constructor() { this.x = 0; } }
class Priv { private toString(): string { return "c"; } }
class Node { val: i32; public next: Node; constructor(val: i32) { this.val = val; } }

console.log(new Point(1, 2));
console.log(new Person("bob", 30));
console.log(new Dog(4, true));
console.log(new Bare());
console.log(new Priv());
console.log("p=" + new Point(7, 8));
let n: Node = new Node(1);
console.log(`node=${n}`);
