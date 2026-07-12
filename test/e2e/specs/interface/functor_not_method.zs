interface Runnable {
  run(): i32;
}

// Task provides `run` as a function-typed field (a functor object), not a method. A field
// cannot satisfy an interface method — dispatch needs a vtable slot — so Task does not
// conform (a clean error rather than a wrong-slot dispatch).
class Task {
  public run: () => i32;
  constructor(f: () => i32) { this.run = f; }
}

function go(r: Runnable): i32 {
  return r.run();
}

function main(): i32 {
  let t: Task = new Task((): i32 => { return 7; });
  return go(t);
}
