// Initializers evaluate in declaration order and can reference earlier fields through `this`:
// radius is set to 3 first, then diameter = this.radius * 2 = 6.
class Circle {
    public radius: i32 = 3;
    public diameter: i32 = this.radius * 2;
}

function main(): i32 {
    let c: Circle = new Circle();
    return c.diameter;
}
