export function reset(state: { count: number }): void {
  void (state.count = 0);
}
