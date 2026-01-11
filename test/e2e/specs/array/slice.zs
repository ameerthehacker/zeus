// Test array slice method
function main(): i32 {
    let arr: i32[] = new i32[];
    arr.push(10);
    arr.push(20);
    arr.push(30);
    arr.push(40);
    arr.push(50);
    
    // Test 1: Basic slice (middle portion)
    let slice1: i32[] = arr.slice(1, 4);
    if (slice1.length != 3) { return 1; }
    if (slice1[0] != 20) { return 2; }
    if (slice1[1] != 30) { return 3; }
    if (slice1[2] != 40) { return 4; }
    
    // Test 2: Slice from beginning
    let slice2: i32[] = arr.slice(0, 2);
    if (slice2.length != 2) { return 5; }
    if (slice2[0] != 10) { return 6; }
    if (slice2[1] != 20) { return 7; }
    
    // Test 3: Slice to end
    let slice3: i32[] = arr.slice(3, 5);
    if (slice3.length != 2) { return 8; }
    if (slice3[0] != 40) { return 9; }
    if (slice3[1] != 50) { return 10; }
    
    // Test 4: Empty slice (start == end)
    let slice4: i32[] = arr.slice(2, 2);
    if (slice4.length != 0) { return 11; }
    
    // Test 5: Original array unchanged
    if (arr.length != 5) { return 12; }
    if (arr[0] != 10) { return 13; }
    
    // Return sum of first slice as success indicator
    let sum: i32 = 0;
    for (let i: i32 = 0; i < slice1.length; i++) {
        sum += slice1[i];
    }
    
    return sum; // Expected: 20+30+40 = 90
}
