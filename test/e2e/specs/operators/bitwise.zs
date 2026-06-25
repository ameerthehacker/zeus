// Test bitwise operators: &, |, ^, ~, <<, >>
function main(): i32 {
    // Basic & (bitwise AND)
    if ((5 & 3) != 1) {
        return 1;
    }
    if ((12 & 10) != 8) {
        return 2;
    }

    // Basic | (bitwise OR)
    if ((5 | 3) != 7) {
        return 3;
    }
    if ((12 | 10) != 14) {
        return 4;
    }

    // Basic ^ (bitwise XOR)
    if ((5 ^ 3) != 6) {
        return 5;
    }
    if ((12 ^ 10) != 6) {
        return 6;
    }

    // ~ (bitwise NOT)
    let a: i32 = 0;
    if (~a != -1) {
        return 7;
    }
    let b: i32 = 5;
    if (~b != -6) {
        return 8;
    }

    // << (left shift)
    if ((1 << 3) != 8) {
        return 9;
    }
    if ((5 << 2) != 20) {
        return 10;
    }

    // >> (right shift)
    if ((16 >> 2) != 4) {
        return 11;
    }
    if ((20 >> 1) != 10) {
        return 12;
    }

    // Precedence: & binds tighter than |
    // 4 | 2 & 3 == 4 | (2 & 3) == 4 | 2 == 6
    if ((4 | 2 & 3) != 6) {
        return 13;
    }

    // Precedence: + binds tighter than &
    // 1 + 2 & 3 == (1 + 2) & 3 == 3 & 3 == 3
    if ((1 + 2 & 3) != 3) {
        return 14;
    }

    // Mask pattern
    let val: i32 = 0xFF;
    let mask: i32 = 0x0F;
    if ((val & mask) != 15) {
        return 15;
    }

    // Shift left and right roundtrip
    let x: i32 = 7;
    if (((x << 4) >> 4) != 7) {
        return 16;
    }

    return 0;
}
