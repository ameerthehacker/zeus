function isPalindrome(s: string): boolean {
    let bytes: u8[] = s;
    let len: i32 = bytes.length;
    let i: i32 = 0;
    while (i < len / 2) {
        if (bytes[i] != bytes[len - 1 - i]) {
            return false;
        }
        i = i + 1;
    }
    return true;
}

function reverseString(s: string): string {
    let bytes: u8[] = s;
    let len: i32 = bytes.length;
    let rev: u8[] = new u8[];
    let i: i32 = len - 1;
    while (i >= 0) {
        rev.push(bytes[i]);
        i = i - 1;
    }
    let result: string = rev;
    return result;
}

function main(): i32 {
    if (!isPalindrome("racecar")) { return 1; }
    if (isPalindrome("hello")) { return 2; }
    if (!isPalindrome("a")) { return 3; }
    if (!isPalindrome("abba")) { return 4; }
    if (isPalindrome("abc")) { return 5; }

    if (!reverseString("hello").equals("olleh")) { return 6; }
    if (!reverseString("a").equals("a")) { return 7; }
    if (!reverseString("ab").equals("ba")) { return 8; }

    return 0;
}
