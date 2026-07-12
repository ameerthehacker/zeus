// A library that declares an interface AND a polymorphic function that dispatches through it.
// The concrete conformers live in *client* modules the library never imports — the itable must
// still cover them (built over the whole program, not just this module's classes).
export interface Shape {
  area(): i32;
}

export function areaOf(s: Shape): i32 {
  return s.area();
}

export interface HasValue {
  value: i32;
}

export function readValue(h: HasValue): i32 {
  return h.value;
}
