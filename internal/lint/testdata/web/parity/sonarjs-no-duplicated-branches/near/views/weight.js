export function weight(kind) {
  if (kind === "a") {
    globalThis.console.info(kind);
    return 1;
  } else if (kind === "b") {
    globalThis.console.info(kind);
    return 2;
  } else if (kind === "c") {
    globalThis.console.info(kind);
    return 3;
  }
  return 0;
}
