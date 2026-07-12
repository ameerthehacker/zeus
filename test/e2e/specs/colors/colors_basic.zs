import { red, bold, bgBlue, green } from "@std/console/colors";

// Verifies ANSI wrapping by byte length (an ESC byte + "[..m" + text + reset). Returns 0 on
// success; distinct sentinels on failure.
function main(): i32 {
    // red("x")     = ESC[31m (5) + "x" (1) + ESC[0m (4) = 10
    if (red("x").length != 10) { return 1; }
    // bold("y")    = ESC[1m  (4) + "y" (1) + ESC[0m (4) =  9
    if (bold("y").length != 9) { return 2; }
    // bgBlue("z")  = ESC[44m (5) + "z" (1) + ESC[0m (4) = 10
    if (bgBlue("z").length != 10) { return 3; }
    // green("")    = ESC[32m (5) + ""  (0) + ESC[0m (4) =  9
    if (green("").length != 9) { return 4; }
    // nested: bold(red("x")) = ESC[1m (4) + red("x") (10) + ESC[0m (4) = 18
    if (bold(red("x")).length != 18) { return 5; }
    return 0;
}
