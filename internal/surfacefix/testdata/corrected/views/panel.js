const manyRows = 9;

export function label(count) {
  if (count === 0) {
    return "empty";
  }
  if (count > manyRows) {
    return "many";
  }
  return "some";
}

export function show(state, rows) {
  state.textContent = label(rows.length);
  state.dataset.empty = String(rows.length === 0);
  return rows.length;
}

const state = document.querySelector("#state");
if (state !== null) {
  show(state, []);
}
