function main(): i32 {
    let name: string = "Zeus";
    let version: string = "1.0";

    // 1. Simple interpolation
    let greeting: string = `Hello, ${name}!`;
    if (!greeting.equals("Hello, Zeus!")) {
        return 1;
    }

    // 2. Multiple interpolations
    let info: string = `${name} v${version}`;
    if (!info.equals("Zeus v1.0")) {
        return 2;
    }

    // 3. No interpolation (plain text)
    let plain: string = `just a plain string`;
    if (!plain.equals("just a plain string")) {
        return 3;
    }

    // 4. Interpolation at start
    let atStart: string = `${name} is great`;
    if (!atStart.equals("Zeus is great")) {
        return 4;
    }

    // 5. Template + concatenation
    let prefix: string = "Compiler: ";
    let full: string = prefix + `${name} ${version}`;
    if (!full.equals("Compiler: Zeus 1.0")) {
        return 5;
    }

    // 6. Nested template (template inside interpolation)
    let a: string = "Hello";
    let nested: string = `${a}!`;
    if (!nested.equals("Hello!")) {
        return 6;
    }

    return 0;
}
