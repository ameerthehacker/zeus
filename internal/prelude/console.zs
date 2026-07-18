// console logging primordial
class Console {
    @extern("zeus_Console_log") public log(message: string): void;
    @extern("zeus_Console_error") public error(message: string): void;
    @extern("zeus_Console_info") public info(message: string): void;
}
