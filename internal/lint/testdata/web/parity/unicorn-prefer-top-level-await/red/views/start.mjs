async function start() {
  return "ready";
}

start().then((state) => globalThis.console.info(state));
