// A primitive satisfies a structural interface by autoboxing into Number/Bool, which conform.
interface Stringable {
    toString(): string;
}

function describe(s: Stringable): string {
    return s.toString();
}

console.log(describe(5));
console.log(describe(true));
