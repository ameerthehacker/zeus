// Test array concat method
function main(): i32 {
    // Test 1: Basic concat of two arrays
    let arr1: i32[] = new i32[];
    arr1.push(1);
    arr1.push(2);
    arr1.push(3);
    
    let arr2: i32[] = new i32[];
    arr2.push(4);
    arr2.push(5);
    
    let result: i32[] = arr1.concat(arr2);
    
    // Verify length
    if (result.length != 5) {
        return 1;
    }
    
    // Verify elements
    if (result[0] != 1) { return 2; }
    if (result[1] != 2) { return 3; }
    if (result[2] != 3) { return 4; }
    if (result[3] != 4) { return 5; }
    if (result[4] != 5) { return 6; }
    
    // Test 2: Concat with empty array
    let empty: i32[] = new i32[];
    let result2: i32[] = arr1.concat(empty);
    if (result2.length != 3) {
        return 7;
    }
    
    // Test 3: Original arrays unchanged
    if (arr1.length != 3) { return 8; }
    if (arr2.length != 2) { return 9; }
    
    // Return sum of concatenated array as success indicator
    let sum: i32 = 0;
    for (let i: i32 = 0; i < result.length; i++) {
        sum += result[i];
    }
    
    return sum; // Expected: 1+2+3+4+5 = 15
}
