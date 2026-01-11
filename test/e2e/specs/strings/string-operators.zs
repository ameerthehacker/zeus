// Test string operators

function main(): i32 {
    let a: string = "hello";
    let b: string = "world";
    let c: string = "hello";

    // Test + operator (concatenation)
    let concat: string = a + " " + b;
    log(concat);

    // Test == operator
    if (a == c) {
        log("a == c: true");
    } else {
        log("a == c: false");
    }

    // Test != operator
    if (a != b) {
        log("a != b: true");
    } else {
        log("a != b: false");
    }

    // Test < operator
    if (a < b) {
        log("a < b: true");
    } else {
        log("a < b: false");
    }

    // Test > operator
    if (b > a) {
        log("b > a: true");
    } else {
        log("b > a: false");
    }

    // Test <= operator
    if (a <= c) {
        log("a <= c: true");
    } else {
        log("a <= c: false");
    }

    // Test >= operator
    if (a >= c) {
        log("a >= c: true");
    } else {
        log("a >= c: false");
    }

    return 0;
}
