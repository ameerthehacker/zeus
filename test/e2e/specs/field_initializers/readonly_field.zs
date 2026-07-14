// A readonly instance field may be given a default via an initializer (the synthesized assignment
// runs in the constructor, where readonly writes are allowed).
class Config {
    public readonly limit: i32 = 7;
}

function main(): i32 {
    let c: Config = new Config();
    return c.limit;
}
