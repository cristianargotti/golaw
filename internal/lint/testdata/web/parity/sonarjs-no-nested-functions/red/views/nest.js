export function outer() {
  return function second() {
    return function third() {
      return function fourth() {
        return function fifth() {
          return function sixth() {
            return 1;
          };
        };
      };
    };
  };
}
