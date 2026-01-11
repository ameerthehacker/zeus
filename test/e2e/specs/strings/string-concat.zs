// Test string concatenation
function main(): i32 {
    let hello: string = "Hello";
    let space: string = " ";
    let world: string = "World";
    let exclaim: string = "!";
    
    // Concatenate strings
    let greeting: string = hello.concat(space);
    let full: string = greeting.concat(world);
    let result: string = full.concat(exclaim);
    
    // Verify the result
    let expected: string = "Hello World!";
    
    if (result.equals(expected)) {
        // Test chained concat
        let chained: string = hello.concat(space).concat(world).concat(exclaim);
        if (chained.equals(expected)) {
            return 0;
        }
        return 2;
    }
    return 1;
}
