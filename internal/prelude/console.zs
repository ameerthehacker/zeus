// console logging primordial
class Console {
    public extern("zeus_Console_log") log(message: string): void;
    public extern("zeus_Console_error") error(message: string): void;
    public extern("zeus_Console_info") info(message: string): void;
}
