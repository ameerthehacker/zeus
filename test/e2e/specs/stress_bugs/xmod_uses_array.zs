// helper: materializes an object-element array type (Widget[]) in an imported module
export class Widget { public id: i32; constructor(id: i32) { this.id = id; } }
export function firstId(xs: Widget[]): i32 { return xs[0].id; }
