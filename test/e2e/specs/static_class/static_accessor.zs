class Config {
    private static _value: i32;

    public static get value(): i32 {
        return Config._value;
    }
    public static set value(v: i32): void {
        Config._value = v;
    }
}
function main(): i32 {
    Config.value = 5;
    return Config.value;
}
