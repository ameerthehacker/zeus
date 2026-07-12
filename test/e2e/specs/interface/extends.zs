interface Named {
  id: i32;
}

interface Entity extends Named {
  score(): i32;
}

// Player must satisfy every member of Entity AND its parent Named:
// id: i32 (from Named) and score(): i32 (from Entity).
class Player {
  public id: i32;

  constructor(i: i32) {
    this.id = i;
  }

  public score(): i32 {
    return this.id;
  }
}

function register(e: Entity): i32 {
  return 0;
}

function main(): i32 {
  let p: Player = new Player(42);

  return register(p) + p.id;
}
