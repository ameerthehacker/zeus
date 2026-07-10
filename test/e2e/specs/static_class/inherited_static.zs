class Base {
    public static value: i32;
}
class Child extends Base { }

function main(): i32 {
    Base.value = 42;
    return Child.value;
}
