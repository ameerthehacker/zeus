export function abs(n: i32): i32 {
    if (n < 0) {
        return -n;
    }
    return n;
}

export function max(a: i32, b: i32): i32 {
    if (a > b) {
        return a;
    }
    return b;
}

export function min(a: i32, b: i32): i32 {
    if (a < b) {
        return a;
    }
    return b;
}

export function clamp(value: i32, low: i32, high: i32): i32 {
    if (value < low) {
        return low;
    }
    if (value > high) {
        return high;
    }
    return value;
}
