// Test array find and findIndex methods
function main(): i32 {
    let numbers: i32[] = new i32[];
    numbers.push(10);
    numbers.push(20);
    numbers.push(30);
    numbers.push(20);  // Duplicate
    numbers.push(40);
    
    // Test 1: findIndex - find first occurrence
    let idx1: i32 = numbers.findIndex(20);
    if (idx1 != 1) { return 1; }
    
    // Test 2: findIndex - not found returns -1
    let idx2: i32 = numbers.findIndex(999);
    if (idx2 != -1) { return 2; }
    
    // Test 3: findIndex - find first element
    let idx3: i32 = numbers.findIndex(10);
    if (idx3 != 0) { return 3; }
    
    // Test 4: findIndex - find last element
    let idx4: i32 = numbers.findIndex(40);
    if (idx4 != 4) { return 4; }
    
    // Test 5: find - returns the element when found
    let found1: i32 = numbers.find(30);
    if (found1 != 30) { return 5; }
    
    // Test 6: find - returns 0 (default) when not found
    let notFound: i32 = numbers.find(999);
    if (notFound != 0) { return 6; }
    
    // Test 7: find - first element
    let found2: i32 = numbers.find(10);
    if (found2 != 10) { return 7; }
    
    // Test 8: Empty array
    let empty: i32[] = new i32[];
    let emptyIdx: i32 = empty.findIndex(10);
    if (emptyIdx != -1) { return 8; }
    
    let emptyFind: i32 = empty.find(10);
    if (emptyFind != 0) { return 9; }
    
    // Return success indicator (sum of first 3 elements)
    return numbers[0] + numbers[1] + numbers[2];  // 10 + 20 + 30 = 60
}
