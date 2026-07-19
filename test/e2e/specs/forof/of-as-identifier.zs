// `of` is a contextual keyword — it remains a valid identifier outside of for...of
function main(): i32 {
    let of: i32 = 5;
    return of - 5;
}
