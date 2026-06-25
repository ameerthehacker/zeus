// Test compound assignment operators: +=, -=, *=, /=, %=
function main(): i32 {
    let a: i32 = 10;
    
    // Test +=
    a += 5;
    if (a != 15) {
        return 1;
    }
    
    // Test -=
    a -= 3;
    if (a != 12) {
        return 2;
    }
    
    // Test *=
    a *= 2;
    if (a != 24) {
        return 3;
    }
    
    // Test /=
    a /= 4;
    if (a != 6) {
        return 4;
    }
    
    // Test %=
    a %= 4;
    if (a != 2) {
        return 5;
    }
    
    // Test compound with expressions
    let b: i32 = 5;
    let c: i32 = 3;
    
    b += c * 2;  // b = 5 + 6 = 11
    if (b != 11) {
        return 6;
    }
    
    b -= c + 1;  // b = 11 - 4 = 7
    if (b != 7) {
        return 7;
    }
    
    // Test chained compound
    let d: i32 = 2;
    d += 3;  // d = 5
    d *= 4;  // d = 20
    d -= 5;  // d = 15
    d /= 3;  // d = 5
    if (d != 5) {
        return 8;
    }

    // Test **=
    let p: i32 = 2;
    p **= 10;  // 2 ** 10 = 1024
    if (p != 1024) {
        return 9;
    }

    // Test &=
    let x: i32 = 0xFF;
    x &= 0x0F;  // 255 & 15 = 15
    if (x != 15) {
        return 10;
    }

    // Test |=
    let y: i32 = 0xF0;
    y |= 0x0F;  // 240 | 15 = 255
    if (y != 255) {
        return 11;
    }

    // Test ^=
    let z: i32 = 0xFF;
    z ^= 0x0F;  // 255 ^ 15 = 240
    if (z != 240) {
        return 12;
    }

    // Test <<=
    let s: i32 = 1;
    s <<= 4;  // 1 << 4 = 16
    if (s != 16) {
        return 13;
    }

    // Test >>=
    let r: i32 = 256;
    r >>= 4;  // 256 >> 4 = 16
    if (r != 16) {
        return 14;
    }

    return 0;
}
