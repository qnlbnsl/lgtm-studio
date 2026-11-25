// The screens build lists as `[cond && item, ...].filter(Boolean)`. TypeScript does not narrow
// that call by itself, so this overload drops the false, null, undefined, 0 and '' members.
interface Array<T> {
  filter(predicate: BooleanConstructor): Exclude<T, false | null | undefined | 0 | ''>[];
}
interface ReadonlyArray<T> {
  filter(predicate: BooleanConstructor): Exclude<T, false | null | undefined | 0 | ''>[];
}
