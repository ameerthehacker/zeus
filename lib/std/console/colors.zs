// Chalk-like terminal colors — import from "@std/console/colors".
//
// Each function wraps its text in an ANSI SGR escape code and a reset, so styles compose by
// nesting: bold(red("fatal")). The reset (\e[0m) clears all attributes; the redundant trailing
// reset produced by nesting is harmless.

// ---- Foreground colors ----
export function black(text: string): string { return "\e[30m" + text + "\e[0m"; }
export function red(text: string): string { return "\e[31m" + text + "\e[0m"; }
export function green(text: string): string { return "\e[32m" + text + "\e[0m"; }
export function yellow(text: string): string { return "\e[33m" + text + "\e[0m"; }
export function blue(text: string): string { return "\e[34m" + text + "\e[0m"; }
export function magenta(text: string): string { return "\e[35m" + text + "\e[0m"; }
export function cyan(text: string): string { return "\e[36m" + text + "\e[0m"; }
export function white(text: string): string { return "\e[37m" + text + "\e[0m"; }
export function gray(text: string): string { return "\e[90m" + text + "\e[0m"; }
export function grey(text: string): string { return "\e[90m" + text + "\e[0m"; }

// ---- Background colors ----
export function bgBlack(text: string): string { return "\e[40m" + text + "\e[0m"; }
export function bgRed(text: string): string { return "\e[41m" + text + "\e[0m"; }
export function bgGreen(text: string): string { return "\e[42m" + text + "\e[0m"; }
export function bgYellow(text: string): string { return "\e[43m" + text + "\e[0m"; }
export function bgBlue(text: string): string { return "\e[44m" + text + "\e[0m"; }
export function bgMagenta(text: string): string { return "\e[45m" + text + "\e[0m"; }
export function bgCyan(text: string): string { return "\e[46m" + text + "\e[0m"; }
export function bgWhite(text: string): string { return "\e[47m" + text + "\e[0m"; }

// ---- Styles ----
export function bold(text: string): string { return "\e[1m" + text + "\e[0m"; }
export function dim(text: string): string { return "\e[2m" + text + "\e[0m"; }
export function italic(text: string): string { return "\e[3m" + text + "\e[0m"; }
export function underline(text: string): string { return "\e[4m" + text + "\e[0m"; }
export function inverse(text: string): string { return "\e[7m" + text + "\e[0m"; }
export function strikethrough(text: string): string { return "\e[9m" + text + "\e[0m"; }
