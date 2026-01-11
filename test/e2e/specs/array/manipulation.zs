// Test array manipulation methods: reverse (non-mutating), fill, clear, isEmpty
function main(): i32 {
    // Test 1: isEmpty on empty array
    let empty: i32[] = new i32[];
    if (!empty.isEmpty()) { return 1; }
    
    // Test 2: isEmpty on non-empty array
    let arr: i32[] = new i32[];
    arr.push(1);
    arr.push(2);
    arr.push(3);
    if (arr.isEmpty()) { return 2; }
    
    // Test 3: reverse - returns NEW array (non-mutating)
    let reversed: i32[] = arr.reverse();
    
    // Check new array is reversed
    if (reversed[0] != 3) { return 3; }
    if (reversed[1] != 2) { return 4; }
    if (reversed[2] != 1) { return 5; }
    
    // Test 4: original array should NOT be modified (non-mutating)
    if (arr[0] != 1) { return 6; }
    if (arr[1] != 2) { return 7; }
    if (arr[2] != 3) { return 8; }
    
    // Test 5: fill - all elements (in-place mutation)
    arr.fill(42);
    if (arr[0] != 42) { return 9; }
    if (arr[1] != 42) { return 10; }
    if (arr[2] != 42) { return 11; }
    if (arr.length != 3) { return 12; }
    
    // Test 6: clear
    arr.clear();
    if (arr.length != 0) { return 13; }
    if (!arr.isEmpty()) { return 14; }
    
    // Test 7: can still push after clear
    arr.push(100);
    if (arr.length != 1) { return 15; }
    if (arr[0] != 100) { return 16; }
    
    // Test 8: reverse single element (should work)
    let singleReversed: i32[] = arr.reverse();
    if (singleReversed[0] != 100) { return 17; }
    
    // Test 9: reverse even number of elements
    let arr2: i32[] = new i32[];
    arr2.push(1);
    arr2.push(2);
    arr2.push(3);
    arr2.push(4);
    let reversed2: i32[] = arr2.reverse();
    
    // Check new array is reversed
    if (reversed2[0] != 4) { return 18; }
    if (reversed2[1] != 3) { return 19; }
    if (reversed2[2] != 2) { return 20; }
    if (reversed2[3] != 1) { return 21; }
    
    // Original should still be intact
    if (arr2[0] != 1) { return 22; }
    if (arr2[3] != 4) { return 23; }
    
    // Test 10: chain reverse().reverse() should give back original order
    let doubleReversed: i32[] = arr2.reverse().reverse();
    if (doubleReversed[0] != 1) { return 24; }
    if (doubleReversed[3] != 4) { return 25; }
    
    // Return 0 for success
    return 0;
}
