// Test array search methods: indexOf, lastIndexOf, includes
function main(): i32 {
    let arr: i32[] = new i32[];
    arr.push(10);
    arr.push(20);
    arr.push(30);
    arr.push(20); // Duplicate
    arr.push(40);
    
    // Test 1: indexOf - first occurrence
    let idx1: i32 = arr.indexOf(20);
    if (idx1 != 1) { return 1; }
    
    // Test 2: indexOf - element at beginning
    let idx2: i32 = arr.indexOf(10);
    if (idx2 != 0) { return 2; }
    
    // Test 3: indexOf - element at end
    let idx3: i32 = arr.indexOf(40);
    if (idx3 != 4) { return 3; }
    
    // Test 4: indexOf - element not found
    let idx4: i32 = arr.indexOf(999);
    if (idx4 != -1) { return 4; }
    
    // Test 5: lastIndexOf - last occurrence of duplicate
    let lastIdx1: i32 = arr.lastIndexOf(20);
    if (lastIdx1 != 3) { return 5; }
    
    // Test 6: lastIndexOf - unique element
    let lastIdx2: i32 = arr.lastIndexOf(30);
    if (lastIdx2 != 2) { return 6; }
    
    // Test 7: lastIndexOf - element not found
    let lastIdx3: i32 = arr.lastIndexOf(999);
    if (lastIdx3 != -1) { return 7; }
    
    // Test 8: includes - element exists
    let has20: boolean = arr.includes(20);
    if (!has20) { return 8; }
    
    // Test 9: includes - element at beginning
    let has10: boolean = arr.includes(10);
    if (!has10) { return 9; }
    
    // Test 10: includes - element at end
    let has40: boolean = arr.includes(40);
    if (!has40) { return 10; }
    
    // Test 11: includes - element not found
    let has999: boolean = arr.includes(999);
    if (has999) { return 11; }
    
    // Return 0 for success
    return 0;
}
