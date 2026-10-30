function render(view) {
  return view.title;
}

const engine = {
  eval(text) {
    return `eval(${text})`;
  },
};

module.exports = { render, engine };
