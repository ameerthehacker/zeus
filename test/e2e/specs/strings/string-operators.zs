// Test string operators

function main(): i32 {
    let a: string = "hello";
    let b: string = "world";
    let c: string = "hello";

    // Test + operator (concatenation)
    let concat: string = a + " " + b;
    console.log(concat);

    // Test == operator
    if (a == c) {
        console.log("a == c: true");
    } else {
        console.log("a == c: false");
    }

    // Test != operator
    if (a != b) {
        console.log("a != b: true");
    } else {
        console.log("a != b: false");
    }

    // Test < operator
    if (a < b) {
        console.log("a < b: true");
    } else {
        console.log("a < b: false");
    }

    // Test > operator
    if (b > a) {
        console.log("b > a: true");
    } else {
        console.log("b > a: false");
    }

    // Test <= operator
    if (a <= c) {
        console.log("a <= c: true");
    } else {
        console.log("a <= c: false");
    }

    // Test >= operator
    if (a >= c) {
        console.log("a >= c: true");
    } else {
        console.log("a >= c: false");
    }

    return 0;
}
