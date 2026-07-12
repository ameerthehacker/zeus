// Arrays now conform to interfaces they structurally match (see array_conforms.zs). But an array is
// still rejected when it is MISSING a required member: Point[] has push/get/length but no area(), so
// it is not assignable to HasArea — a clean compile error, never a bad dispatch.
class Point {
  public x: i32;
  constructor(x: i32) { this.x = x; }
}

interface HasArea {
  area(): i32;
}

function useArea(h: HasArea): i32 {
  return h.area();
}

function main(): i32 {
  let a: Point[] = new Point[];
  a.push(new Point(7));
  let h: HasArea = a;   // error: 'Point[]' is not assignable to type 'HasArea'
  return useArea(h);
}
