export function parse(text) {
  try {
    const value = JSON.parse(text);
    return value;
  } catch (error) {
    globalThis.console.error(error);
  }
  return null;
}
