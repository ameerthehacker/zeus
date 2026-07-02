function main(): i32 {
  let obj = new (class {
    public v: i32;
    constructor(v: i32) { this.v = v; }
    get(): i32 { return this.v; }
  })(42);
  return obj.get();
}
