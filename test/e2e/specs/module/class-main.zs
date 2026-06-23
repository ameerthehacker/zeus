import { Counter } from "./counter";

function main(): i32 {
    let c: Counter = new Counter();
    return c.value;
}
