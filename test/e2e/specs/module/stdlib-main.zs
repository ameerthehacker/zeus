import { red, bold } from "@std/console/colors";

function main(): i32 {
    // red("x")  = ESC[31m (5) + "x" (1) + ESC[0m (4) = 10 bytes
    // bold("y") = ESC[1m  (4) + "y" (1) + ESC[0m (4) =  9 bytes
    return red("x").length + bold("y").length; // 19
}
