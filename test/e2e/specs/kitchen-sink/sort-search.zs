function insertionSort(arr: i32[]): void {
    let n: i32 = arr.length;
    for (let i: i32 = 1; i < n; i++) {
        let key: i32 = arr[i];
        let j: i32 = i - 1;
        let insertPos: i32 = 0;
        while (j >= 0) {
            if (arr[j] > key) {
                arr.set(j + 1, arr[j]);
                j = j - 1;
            } else {
                insertPos = j + 1;
                j = -1;
            }
        }
        arr.set(insertPos, key);
    }
}

function binarySearch(arr: i32[], target: i32): i32 {
    let low: i32 = 0;
    let high: i32 = arr.length - 1;
    while (low <= high) {
        let mid: i32 = (low + high) / 2;
        if (arr[mid] == target) {
            return mid;
        }
        if (arr[mid] < target) {
            low = mid + 1;
        } else {
            high = mid - 1;
        }
    }
    return -1;
}

function main(): i32 {
    let arr: i32[] = new i32[];
    arr.push(5);
    arr.push(2);
    arr.push(8);
    arr.push(1);
    arr.push(9);
    arr.push(3);

    insertionSort(arr);

    if (arr[0] != 1) { return 1; }
    if (arr[1] != 2) { return 2; }
    if (arr[2] != 3) { return 3; }
    if (arr[3] != 5) { return 4; }
    if (arr[4] != 8) { return 5; }
    if (arr[5] != 9) { return 6; }

    if (binarySearch(arr, 3) != 2) { return 7; }
    if (binarySearch(arr, 1) != 0) { return 8; }
    if (binarySearch(arr, 9) != 5) { return 9; }
    if (binarySearch(arr, 7) != -1) { return 10; }

    return 0;
}
